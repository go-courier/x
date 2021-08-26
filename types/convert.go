package types

import (
	"go/importer"
	"go/token"
	"go/types"
	"reflect"
	"sync"
)

var (
	typesCache = sync.Map{}
	pkgCache   = sync.Map{}
)

func NewPackage(importPath string) *types.Package {
	if v, ok := pkgCache.Load(importPath); ok {
		return v.(*types.Package)
	}
	pkg, err := importer.ForCompiler(token.NewFileSet(), "source", nil).Import(importPath)
	if err != nil && importPath != "" {
		panic(err)
	}
	pkgCache.Store(importPath, pkg)
	return pkg
}

func TypeByName(importPath string, name string) types.Type {
	pkg := NewPackage(importPath)
	if pkg == nil {
		return nil
	}
	return pkg.Scope().Lookup(name).Type()
}

func NewTypesTypeFromReflectType(rtype reflect.Type) types.Type {
	underlying := func() types.Type {
		switch rtype.Kind() {
		case reflect.Array:
			return types.NewArray(NewTypesTypeFromReflectType(rtype.Elem()), int64(rtype.Len()))
		case reflect.Slice:
			return types.NewSlice(NewTypesTypeFromReflectType(rtype.Elem()))
		case reflect.Map:
			return types.NewMap(NewTypesTypeFromReflectType(rtype.Key()), NewTypesTypeFromReflectType(rtype.Elem()))
		case reflect.Chan:
			return types.NewChan(types.ChanDir(rtype.ChanDir()), NewTypesTypeFromReflectType(rtype.Elem()))
		case reflect.Func:
			params := make([]*types.Var, rtype.NumIn())
			for i := range params {
				param := rtype.In(i)
				params[i] = types.NewParam(0, NewPackage(param.PkgPath()), "", NewTypesTypeFromReflectType(param))
			}
			results := make([]*types.Var, rtype.NumOut())
			for i := range results {
				result := rtype.Out(i)
				results[i] = types.NewParam(0, NewPackage(result.PkgPath()), "", NewTypesTypeFromReflectType(result))
			}
			return types.NewSignature(
				nil,
				types.NewTuple(params...),
				types.NewTuple(results...),
				rtype.IsVariadic(),
			)
		case reflect.Interface:
			funcs := make([]*types.Func, rtype.NumMethod())
			for i := range funcs {
				m := rtype.Method(i)

				funcs[i] = types.NewFunc(
					0,
					NewPackage(m.PkgPath),
					m.Name,
					NewTypesTypeFromReflectType(m.Type).(*types.Signature),
				)
			}
			return types.NewInterfaceType(funcs, nil).Complete()
		case reflect.Struct:
			fields := make([]*types.Var, rtype.NumField())
			tags := make([]string, len(fields))
			for i := range fields {
				f := rtype.Field(i)
				fields[i] = types.NewField(
					0,
					NewPackage(f.PkgPath),
					f.Name,
					NewTypesTypeFromReflectType(f.Type),
					f.Anonymous,
				)
				tags[i] = string(f.Tag)
			}
			return types.NewStruct(fields, tags)
		case reflect.Bool:
			return types.Typ[types.Bool]
		case reflect.Int:
			return types.Typ[types.Int]
		case reflect.Int8:
			return types.Typ[types.Int8]
		case reflect.Int16:
			return types.Typ[types.Int16]
		case reflect.Int32:
			return types.Typ[types.Int32]
		case reflect.Int64:
			return types.Typ[types.Int64]
		case reflect.Uint:
			return types.Typ[types.Uint]
		case reflect.Uint8:
			return types.Typ[types.Uint8]
		case reflect.Uint16:
			return types.Typ[types.Uint16]
		case reflect.Uint32:
			return types.Typ[types.Uint32]
		case reflect.Uint64:
			return types.Typ[types.Uint64]
		case reflect.Uintptr:
			return types.Typ[types.Uintptr]
		case reflect.Float32:
			return types.Typ[types.Float32]
		case reflect.Float64:
			return types.Typ[types.Float64]
		case reflect.Complex64:
			return types.Typ[types.Complex64]
		case reflect.Complex128:
			return types.Typ[types.Complex128]
		case reflect.String:
			return types.Typ[types.String]
		case reflect.UnsafePointer:
			return types.Typ[types.UnsafePointer]
		}
		return nil
	}

	ptrCount := 0

	mayWithPtr := func(typ types.Type) types.Type {
		for ptrCount > 0 {
			typ = types.NewPointer(typ)
			ptrCount--
		}
		return typ
	}

	for rtype.Kind() == reflect.Ptr {
		rtype = rtype.Elem()
		ptrCount++
	}

	name := rtype.Name()
	pkgPath := rtype.PkgPath()

	if name == "error" && pkgPath == "" {
		return mayWithPtr(TypeByName("errors", "New").Underlying().(*types.Signature).Results().At(0).Type())
	}

	if pkgPath != "" {
		key := name
		if pkgPath != "" {
			key = pkgPath + "." + name
		}

		if typ, ok := typesCache.Load(key); ok {
			return mayWithPtr(typ.(types.Type))
		}

		ttype := TypeByName(pkgPath, name)
		typesCache.Store(key, ttype)
		return mayWithPtr(ttype)
	}

	return mayWithPtr(underlying())
}
