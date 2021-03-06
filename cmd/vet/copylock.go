// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains the code to check that locks are not passed by value.

package main

import (
	"bytes"
	"fmt"
	"go/ast"

	"github.com/polaris1119/go.tools/go/types"
)

func init() {
	register("copylocks",
		"check that locks are not passed by value",
		checkCopyLocks,
		funcDecl)
}

// checkCopyLocks checks whether a function might
// inadvertently copy a lock, by checking whether
// its receiver, parameters, or return values
// are locks.
func checkCopyLocks(f *File, node ast.Node) {
	d := node.(*ast.FuncDecl)

	if d.Recv != nil && len(d.Recv.List) > 0 {
		expr := d.Recv.List[0].Type
		if path := lockPath(f.pkg.typesPkg, f.pkg.types[expr].Type); path != nil {
			f.Badf(expr.Pos(), "%s passes Lock by value: %v", d.Name.Name, path)
		}
	}

	if d.Type.Params != nil {
		for _, field := range d.Type.Params.List {
			expr := field.Type
			if path := lockPath(f.pkg.typesPkg, f.pkg.types[expr].Type); path != nil {
				f.Badf(expr.Pos(), "%s passes Lock by value: %v", d.Name.Name, path)
			}
		}
	}

	if d.Type.Results != nil {
		for _, field := range d.Type.Results.List {
			expr := field.Type
			if path := lockPath(f.pkg.typesPkg, f.pkg.types[expr].Type); path != nil {
				f.Badf(expr.Pos(), "%s returns Lock by value: %v", d.Name.Name, path)
			}
		}
	}
}

type typePath []types.Type

// pathString pretty-prints a typePath.
func (path typePath) String() string {
	n := len(path)
	var buf bytes.Buffer
	for i := range path {
		if i > 0 {
			fmt.Fprint(&buf, " contains ")
		}
		// The human-readable path is in reverse order, outermost to innermost.
		fmt.Fprint(&buf, path[n-i-1].String())
	}
	return buf.String()
}

// lockPath returns a typePath describing the location of a lock value
// contained in typ. If there is no contained lock, it returns nil.
func lockPath(tpkg *types.Package, typ types.Type) typePath {
	if typ == nil {
		return nil
	}

	// We're only interested in the case in which the underlying
	// type is a struct. (Interfaces and pointers are safe to copy.)
	styp, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	// We're looking for cases in which a reference to this type
	// can be locked, but a value cannot. This differentiates
	// embedded interfaces from embedded values.
	if plock := types.NewMethodSet(types.NewPointer(typ)).Lookup(tpkg, "Lock"); plock != nil {
		if lock := types.NewMethodSet(typ).Lookup(tpkg, "Lock"); lock == nil {
			return []types.Type{typ}
		}
	}

	nfields := styp.NumFields()
	for i := 0; i < nfields; i++ {
		ftyp := styp.Field(i).Type()
		subpath := lockPath(tpkg, ftyp)
		if subpath != nil {
			return append(subpath, typ)
		}
	}

	return nil
}
