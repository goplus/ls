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

package gop

import (
	"io/fs"
	"testing"
)

func file(text string) File {
	return &FileImpl{Content: []byte(text)}
}

func TestBasic(t *testing.T) {
	proj := NewProject(nil, map[string]File{
		"main.spx": file("echo 100"),
		"bar.spx":  file("echo 200"),
	}, FeatAll)
	f, err := proj.AST("main.spx")
	if err != nil || f == nil {
		t.Fatal(err)
	}
	if body := f.ShadowEntry.Body.List; len(body) != 1 {
		t.Fatal("body:", body)
	}
	pkg, err := proj.ASTPackage()
	if err != nil {
		t.Fatal("ASTPackage:", err)
	}
	if pkg.Name != "main" || len(pkg.Files) != 2 {
		t.Fatal("pkg.Name:", pkg.Name, "Files:", len(pkg.Files))
	}
	doc, err := proj.PkgDoc()
	if err != nil {
		t.Fatal("PkgDoc:", err)
	}
	if doc.Name != "main" || len(doc.Funcs) != 0 {
		t.Fatal("doc.Name:", doc.Name, "Funcs:", len(doc.Funcs))
	}
	proj2 := proj.Snapshot()
	f2, err2 := proj2.AST("main.spx")
	if f2 != f || err2 != nil {
		t.Fatal("Snapshot:", f2, err2)
	}
	proj.DeleteFile("main.spx")
	f3, err3 := proj.AST("main.spx")
	if f3 != nil || err3 != fs.ErrNotExist {
		t.Fatal("DeleteFile:", f3, err3)
	}
	f4, err4 := proj2.AST("main.spx")
	if f4 != f || err4 != nil {
		t.Fatal("Snapshot after DeleteFile:", f4, err4)
	}
	if err5 := proj.DeleteFile("main.spx"); err5 != fs.ErrNotExist {
		t.Fatal("DeleteFile after DeleteFile:", err5)
	}
	proj2.Rename("main.spx", "foo.spx")
	f5, err5 := proj2.AST("foo.spx")
	if f5 == f4 || err5 != nil {
		t.Fatal("AST after Rename:", f5, err5)
	}
	if err6 := proj2.Rename("main.spx", "foo.spx"); err6 != fs.ErrNotExist {
		t.Fatal("Rename after Rename:", err6)
	}
	if err7 := proj2.Rename("foo.spx", "bar.spx"); err7 != fs.ErrExist {
		t.Fatal("Rename exists:", err7)
	}
}

func TestNewNil(t *testing.T) {
	proj := NewProject(nil, nil, FeatAll)
	proj.PutFile("main.gop", file("echo 100"))
	f, err := proj.AST("main.gop")
	if err != nil || f == nil {
		t.Fatal(err)
	}
	if body := f.ShadowEntry.Body.List; len(body) != 1 {
		t.Fatal("body:", body)
	}
	if _, files, err := proj.ASTFiles(); err != nil || len(files) != 1 {
		t.Fatal("ASTFiles:", files, err)
	}
	pkg, _, err, _ := proj.TypeInfo()
	if err != nil {
		t.Fatal("TypeInfo:", err)
	}
	if o := pkg.Scope().Lookup("main"); o == nil {
		t.Fatal("Scope.Lookup main failed")
	}
	pkg2, _, err2, _ := proj.Snapshot().TypeInfo()
	if pkg2 != pkg || err2 != err {
		t.Fatal("Snapshot TypeInfo:", pkg2, err2)
	}
	if _, e := proj.Cache("unknown"); e != ErrUnknownKind {
		t.Fatal("Cache unknown:", e)
	}
	proj.RangeFileContents(func(path string, file File) bool {
		if path != "main.gop" {
			t.Fatal("RangeFileContents:", path)
		}
		return true
	})
}

func TestNewCallback(t *testing.T) {
	proj := NewProject(nil, func() map[string]File {
		return map[string]File{
			"main.spx": file("echo 100"),
		}
	}, FeatAll)
	f, err := proj.AST("main.spx")
	if err != nil || f == nil {
		t.Fatal(err)
	}
	if body := f.ShadowEntry.Body.List; len(body) != 1 {
		t.Fatal("body:", body)
	}
	if _, err = proj.FileCache("unknown", "main.spx"); err != ErrUnknownKind {
		t.Fatal("FileCache:", err)
	}
}

func TestErr(t *testing.T) {
	proj := NewProject(nil, map[string]File{
		"main.spx": file("100_err"),
	}, FeatAll)
	if _, err := proj.AST("main.spx"); err == nil {
		t.Fatal("AST no error?")
	}
	if _, err2 := proj.Snapshot().AST("main.spx"); err2 == nil {
		t.Fatal("Snapshot AST no error?")
	}
	if _, _, err3 := proj.ASTFiles(); err3 == nil {
		t.Fatal("ASTFiles no error?")
	}
	proj.PutFile("main.spx", file("echo 100"))
	if _, _, err4, _ := proj.TypeInfo(); err4 == nil {
		t.Fatal("TypeInfo no error?")
	}

	proj = NewProject(nil, map[string]File{
		"main.spx": file("100_err"),
	}, 0)
	if _, _, err5, _ := proj.TypeInfo(); err5 != ErrUnknownKind {
		t.Fatal("TypeInfo:", err5)
	}
	_, err := proj.ASTPackage()
	if err == nil || err.Error() != "unknown kind" {
		t.Fatal("ASTPackage:", err)
	}
	_, err = proj.PkgDoc()
	if err != ErrUnknownKind {
		t.Fatal("PkgDoc:", err)
	}
	_, err = buildPkgDoc(proj)
	if err == nil || err.Error() != "unknown kind" {
		t.Fatal("buildPkgDoc:", err)
	}
}

func TestUpdateFiles(t *testing.T) {
	// Initial project with two files
	proj := NewProject(nil, map[string]File{
		"main.spx": file("echo 100"),
		"bar.spx":  file("echo 200"),
	}, FeatAll)

	// Create new files map with one existing file modified and one new file
	newFiles := map[string]File{
		"main.spx":  file("echo 300"), // Modified file
		"third.spx": file("echo 400"), // New file
		// bar.spx will be deleted
	}

	// Update all files
	proj.UpdateFiles(newFiles)

	// Test deleted file
	if f, err := proj.AST("bar.spx"); f != nil || err != fs.ErrNotExist {
		t.Fatal("Expected bar.spx to be deleted, got:", f, err)
	}

	// Test modified file
	f1, err1 := proj.AST("main.spx")
	if err1 != nil || f1 == nil {
		t.Fatal("Failed to get modified file main.spx:", err1)
	}
	if val, ok := proj.files.Load("main.spx"); !ok || string(val.(File).Content) != "echo 300" {
		t.Fatal("main.spx content not updated correctly")
	}

	// Test new file
	f2, err2 := proj.AST("third.spx")
	if err2 != nil || f2 == nil {
		t.Fatal("Failed to get new file third.spx:", err2)
	}
	if val, ok := proj.files.Load("third.spx"); !ok || string(val.(File).Content) != "echo 400" {
		t.Fatal("third.spx content not set correctly")
	}

	// Verify total number of files
	fileCount := 0
	proj.RangeFiles(func(path string) bool {
		fileCount++
		return true
	})
	if fileCount != 2 {
		t.Fatal("Expected 2 files after update, got:", fileCount)
	}
}
