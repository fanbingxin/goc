package main

import (
        "bytes"
        "flag"
        "fmt"
        "go/ast"
        "go/parser"
        "go/token"
        "log"
        "os"
        "strconv"
        "strings"
)

type Printer struct {
        bytes.Buffer
        indent int
}

func NewPrinter() *Printer {
        return &Printer{}
}

func (p *Printer) P(f string, l ...interface{}) {
        fmt.Fprintf(p, f, l...)
}

func (p *Printer) Pi(f string, l ...interface{}) {
        for i := 0; i < p.indent; i++ {
                fmt.Fprint(p, "    ")
        }
        fmt.Fprintf(p, f, l...)
}

func (p *Printer) Pln(f string, l ...interface{}) {
        p.Pi(f, l...)
        fmt.Fprintln(p)
}

func (p *Printer) Indent() {
        p.indent++
}

func (p *Printer) Unindent() {
        if p.indent > 0 {
                p.indent--
        }
}

func VisitBinExpr(p *Printer, n *ast.BinaryExpr) {
        VisitExpr(p, n.X)
        p.P(n.Op.String())
        VisitExpr(p, n.Y)
}

func VisitExpr(p *Printer, n ast.Expr) {
        switch t := n.(type) {
        case *ast.BasicLit:
                p.P("%s", t.Value)
        case *ast.Ident:
                p.P(t.Name)
        case *ast.SelectorExpr:
                VisitExpr(p, t.X)
                p.P(".%s", t.Sel.Name)
        case *ast.BinaryExpr:
                p.P("(")
                VisitBinExpr(p, t)
                p.P(")")
        case *ast.CallExpr:
                VisitExpr(p, t.Fun)
                params := make([]string, 0)
                for _, arg := range t.Args {
                        sp := new(Printer)
                        VisitExpr(sp, arg)
                        params = append(params, string(sp.Bytes()))
                }
                p.P("(%s)", strings.Join(params, ", "))
        }
}

func VisitBlockStmt(p *Printer, n *ast.BlockStmt) {
        p.Pln("{")
        p.Indent()
        for _, elem := range n.List {
                sem := ";"
                sp := new(Printer)
                switch t := elem.(type) {
                case *ast.ExprStmt:
                        VisitExpr(sp, t.X)
                case *ast.AssignStmt:
                        VisitExpr(sp, t.Lhs[0])
                        sp.P(" %s ", t.Tok.String())
                        VisitExpr(sp, t.Rhs[0])
                case *ast.DeclStmt:
                        VisitDecl(sp, t.Decl)
                case *ast.ReturnStmt:
                        sp.P("return ")
                        VisitExpr(sp, t.Results[0])
                case *ast.IfStmt:
                        sem = ""
                        p.Pi("if (")
                        VisitExpr(p, t.Cond)
                        p.P(")\n")
                        VisitBlockStmt(p, t.Body)
                        if t.Else != nil {
                                p.Pln("else")
                                VisitBlockStmt(p, t.Else.(*ast.BlockStmt))
                        }
                case *ast.ForStmt:
                        sem = ""
                        p.Pi("while (")
                        VisitExpr(p, t.Cond)
                        p.P(")\n")
                        VisitBlockStmt(p, t.Body)
                }
                if sp.Len() > 0 {
                        p.Pln("%s%s", sp.Bytes(), sem)
                }
        }
        p.Unindent()
        p.Pln("}")
}

func VisitFunction(p *Printer, n *ast.FuncDecl) {
        fun := n.Type
        if fun.Results.NumFields() > 1 {
                log.Fatal("number of return error")
        }
        funcname := n.Name.Name
        rettyp := "void"
        if fun.Results.NumFields() > 0 {
                rettyp = fun.Results.List[0].Type.(*ast.Ident).Name
        }

        params := ""
        if fun.Params.NumFields() != 0 {
                paraml := make([]string, 0)
                for _, f := range fun.Params.List {
                        param := f.Type.(*ast.Ident).Name + " " + f.Names[0].Name
                        paraml = append(paraml, param)
                }
                params = strings.Join(paraml, ", ")
        }

        p.Pln("%s %s(%s)", rettyp, funcname, params)
        VisitBlockStmt(p, n.Body)
}

func VisitSpec(p *Printer, n ast.Spec) {
        switch d := n.(type) {
        case *ast.ValueSpec:
                VisitExpr(p, d.Type)
                p.P(" ")
                p.P(d.Names[0].Name)
        case *ast.ImportSpec:
                path, _ := strconv.Unquote(d.Path.Value)
                p.Pln(`#include <%s.h>`, path)
        }
}

func VisitDecl(p *Printer, n ast.Decl) {
        switch d := n.(type) {
        case *ast.FuncDecl:
                VisitFunction(p, d)
        case *ast.GenDecl:
                VisitSpec(p, d.Specs[0])
        default:
                log.Fatalf("unsupport declear type %p", d)
        }
}

func VisitFile(p *Printer, n *ast.File) {
        for _, decl := range n.Decls {
                VisitDecl(p, decl)
        }
}

func main() {
        flag.Parse()
        if flag.NArg() < 1 {
                log.Fatal("missing source file")
        }
        src := flag.Args()[0]
        fset := token.NewFileSet()
        f, err := parser.ParseFile(fset, src, nil, 0)
        if err != nil {
                log.Fatal(err)
        }
        p := NewPrinter()
        VisitFile(p, f)
        p.WriteTo(os.Stdout)
}
