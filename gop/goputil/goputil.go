/*
 * Copyright (c) 2025 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package goputil

import (
	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/token"
	"github.com/goplus/goxlsw/gop"
)

// ClassFieldsDecl returns the class fields declaration.
func ClassFieldsDecl(f *ast.File) *ast.GenDecl {
	if f.IsClass {
		for _, decl := range f.Decls {
			if g, ok := decl.(*ast.GenDecl); ok {
				if g.Tok == token.VAR {
					return g
				}
				continue
			}
			break
		}
	}
	return nil
}

// RangeASTSpecs iterates all Go+ AST specs.
func RangeASTSpecs(proj *gop.Project, tok token.Token, f func(spec ast.Spec)) {
	proj.RangeASTFiles(func(_ string, file *ast.File) {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok && decl.Tok == tok {
				for _, spec := range decl.Specs {
					f(spec)
				}
			}
		}
	})
}

// IsShadow checks if the ident is shadowed.
func IsShadow(proj *gop.Project, ident *ast.Ident) (shadow bool) {
	proj.RangeASTFiles(func(_ string, file *ast.File) {
		if e := file.ShadowEntry; e != nil {
			if e.Name == ident {
				shadow = true
			}
		}
	})
	return
}
