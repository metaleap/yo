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
			println(">>>>>>>" + src_dir_path + "<<<<<<<<<<<<<")
			break
		}
	}
	if src_dir_path == "" {
		panic("no source dir found for " + pkgPath)
	}

	out_file_path := filepath.Join(src_dir_path, "ˍdbcodegen.go")
	var buf str.Buf
	buf.WriteString("package ")
	buf.WriteString(pkg_name)
	buf.WriteString("\n\nimport q \"yo/db/query\"\n\n")
	for _, desc := range descs {
		// render enumerants for the column names
		buf.WriteString("type ")
		buf.WriteString(desc.ty.Name())
		buf.WriteString("Col = q.C\n\n")
		buf.WriteString("const (\n")
		for i, col_name := range desc.cols {
			buf.WriteByte('\t')
			buf.WriteString(desc.ty.Name())
			buf.WriteString(str.Up(desc.fields[i][:1]))
			buf.WriteString(desc.fields[i][1:])
			buf.WriteString(" = ")
			buf.WriteString(desc.ty.Name())
			buf.WriteString("Col(\"")
			buf.WriteString(string(col_name))
			buf.WriteString("\")\n")
		}
		buf.WriteString(")\n\n")

		// render querying-payload struct
		//   { "OR": [ {"EQ":[{"F":"Id"},{"F":"EmailAddr"}]}, {"AND": [{"LT":[{"F":"Id"},{"N":123} ] }  , {"NOT": { "IN": [ {"N":321}, {"N":456}] } } ] } ] }

		// BizObjQueryValue: {
		//		F?: "Field1" | "Field2"
		//		S?: string
		//		N?: number
		//		B?: boolean | BizObjQueryExpr
		//		V?: BizObjQueryValue
		// }
		// BizObjQueryExpr: {
		//		AND:	[BizObjQueryExpr]
		//		OR		[BizObjQueryExpr]
		//		NOT		BizObjQueryExpr
		//		EQ		BizObjQueryValue
		//		IN		[BizObjQueryValue]
		// }
	}
	raw_src, err := format.Source([]byte(buf.String()))
	if err != nil {
		panic(err)
	}

	if old_src, _ := os.ReadFile(out_file_path); !bytes.Equal(old_src, raw_src) {
		if err = os.WriteFile(out_file_path, raw_src, os.ModePerm); err != nil {
			panic(err)
		}
		return true
	}
	return false
}
