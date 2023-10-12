package yodb

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"

	. "yo/util"
	"yo/util/str"
)

func codeGenDBStructs() {
	if !IsDevMode {
		return
	}
	by_pkg_path := map[string][]*structDesc{}
	for _, desc := range ensureDescs {
		pkg_path := desc.ty.PkgPath()
		by_pkg_path[pkg_path] = append(by_pkg_path[pkg_path], desc)
	}
	did_code_gen := false
	for pkg_path, descs := range by_pkg_path {
		did_code_gen = (codeGenDBStructsFor(pkg_path, descs)) || did_code_gen
	}
	if did_code_gen {
		panic("dbcodegen'd, please restart")
	}
}

func codeGenDBStructsFor(pkgPath string, descs []*structDesc) bool {
	var src_dir_path, pkg_name string
	for _, desc := range descs { // find src_dir_path in which to generate `ˍcodegend.go`
		found, needle := map[string]string{}, []byte("\ntype "+desc.ty.Name()+" struct {\n\t")

		WalkCodeFiles(
			(str.Begins(desc.ty.PkgPath(), "yo/") || (desc.ty.PkgPath() == "yo")),
			(str.Begins(desc.ty.PkgPath(), "main/") || (desc.ty.PkgPath() == "main")),
			func(path string, dirEntry fs.DirEntry) {
				if str.Ends(path, ".go") {
					data, err := os.ReadFile(path)
					if err != nil {
						panic(err)
					}
					if dir_path, idx := filepath.Dir(path), bytes.Index(data, needle); idx > 0 {
						if idx = bytes.IndexByte(data, '\n'); (idx < len("package ")) || !bytes.Equal(data[0:len("package ")], []byte("package ")) {
							panic("no package name for " + pkgPath)
						}
						pkg_name = str.Trim(string(data[len("package "):idx]))
						found[dir_path] = path
					}
				}
			},
		)
		if len(found) == 0 {
			panic("no source dir found for " + desc.ty.PkgPath() + "." + desc.ty.Name())
		} else if len(found) > 1 {
			panic("too many source dirs found for " + desc.ty.PkgPath() + "." + desc.ty.Name())
		}
		for src_dir_path = range found {
			break
		}
	}
	if src_dir_path == "" {
		panic("no source dir found for " + pkgPath)
	}

	out_file_path := filepath.Join(src_dir_path, "ˍdbcodegen.go")
	old_src, old_src_err := os.ReadFile(out_file_path)
	if len(descs) == 0 {
		if old_src_err == os.ErrNotExist {
			return false
		} else {
			_ = os.Remove(out_file_path)
			return true
		}
	}

	var buf str.Buf
	buf.WriteString("package ")
	buf.WriteString(pkg_name)
	buf.WriteString("\n\nimport q \"yo/db/query\"\n\n")
	for _, desc := range descs {
		// render enumerants for the column names
		codeGenWriteEnumDecl(&buf, desc, "Col", "q.C", true)
		// render enumerants for the field names
		codeGenWriteEnumDecl(&buf, desc, "Field", "q.F", false)

		for _, method := range [][3]string{
			{"Asc", "()q.OrderBy", "()"},
			{"Desc", "()q.OrderBy", "()"},
			{"In", "(set...any)q.Query", "(set...)"},
			{"NotIn", "(set...any)q.Query", "(set...)"},
			{"Equal", "(other any)q.Query", "(other)"},
			{"NotEqual", "(other any)q.Query", "(other)"},
			{"LessThan", "(other any)q.Query", "(other)"},
			{"GreaterThan", "(other any)q.Query", "(other)"},
			{"LessOrEqual", "(other any)q.Query", "(other)"},
			{"GreaterOrEqual", "(other any)q.Query", "(other)"},
		} {
			buf.WriteString("func(me ")
			buf.WriteString(desc.ty.Name())
			buf.WriteString("Field) ")
			buf.WriteString(method[0])
			buf.WriteString(method[1])
			buf.WriteString("{return ((q.F)(me)).")
			buf.WriteString(method[0])
			buf.WriteString(method[2])
			buf.WriteString("}\n")
		}
	}
	raw_src, err := format.Source([]byte(buf.String()))
	if err != nil {
		panic(err)
	}

	if !bytes.Equal(old_src, raw_src) {
		if err = os.WriteFile(out_file_path, raw_src, os.ModePerm); err != nil {
			panic(err)
		}
		return true
	}
	return false
}

func codeGenWriteEnumDecl(buf *str.Buf, desc *structDesc, name string, goTypeAliasOf string, isForCols bool) {
	buf.WriteString("type ")
	buf.WriteString(desc.ty.Name())
	buf.WriteString(name)
	buf.WriteString(If(isForCols, " = ", " "))
	buf.WriteString(goTypeAliasOf)
	buf.WriteString("\n\n")
	buf.WriteString("const (\n")
	for i, col_name := range desc.cols {
		field_name := desc.fields[i]
		buf.WriteByte('\t')
		if isForCols || !str.IsLo(string(field_name[:1])) {
			buf.WriteString(desc.ty.Name())
		} else {
			buf.WriteString(str.Lo(desc.ty.Name()[:1]))
			buf.WriteString(desc.ty.Name()[1:])
		}
		buf.WriteString(name)
		buf.WriteString(str.Up(string(desc.fields[i][:1])))
		buf.WriteString(string(desc.fields[i][1:]))
		if !isForCols {
			buf.WriteString(" ")
			buf.WriteString(desc.ty.Name())
			buf.WriteString(name)
		}
		buf.WriteString(" = ")
		if isForCols {
			buf.WriteString(desc.ty.Name())
			buf.WriteString(name)
			buf.WriteString("(\"")
			buf.WriteString(string(col_name))
			buf.WriteString("\")\n")
		} else {
			buf.WriteString("\"")
			buf.WriteString(string(field_name))
			buf.WriteString("\"\n")
		}
	}
	buf.WriteString(")\n\n")
}
