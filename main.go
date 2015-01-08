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

var (
        printAST = flag.Bool("ast", false, "print ast")
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

func expr(n ast.Expr) string {
        p := new(Printer)
        VisitExpr(p, n)
        return p.String()
}

func field(n *ast.Field) string {
        p := new(Printer)
        if t, ok := n.Type.(*ast.StarExpr); ok {
                p.P("%s* %s", expr(t.X), n.Names[0].Name)
        } else {
                p.P("%s %s", expr(n.Type), n.Names[0].Name)
        }
        return p.String()
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
        case *ast.UnaryExpr:
                p.P(t.Op.String())
                VisitExpr(p, t.X)
        case *ast.StarExpr:
                p.P("*")
                VisitExpr(p, t.X)
        case *ast.IndexExpr:
                VisitExpr(p, t.X)
                p.P("[")
                VisitExpr(p, t.Index)
                p.P("]")
        case *ast.CallExpr:
                VisitExpr(p, t.Fun)
                params := make([]string, 0)
                for _, arg := range t.Args {
                        params = append(params, expr(arg))
                }
                p.P("(%s)", strings.Join(params, ", "))
        }
}

func VisitStmt(p *Printer, n ast.Stmt) {
        switch t := n.(type) {
        case *ast.ExprStmt:
                p.Pln("%s;", expr(t.X))
        case *ast.AssignStmt:
                p.Pln("%s %s %s;", expr(t.Lhs[0]), t.Tok.String(), expr(t.Rhs[0]))
        case *ast.DeclStmt:
                VisitDecl(p, t.Decl)
        case *ast.ReturnStmt:
                p.Pln("return %s;", expr(t.Results[0]))
        case *ast.IncDecStmt:
                VisitExpr(p, t.X)
                p.Pln("%s;", t.Tok.String())
        case *ast.IfStmt:
                p.Pi("if (")
                VisitExpr(p, t.Cond)
                p.P(") ")
                VisitBlockStmt(p, t.Body)
                if t.Else != nil {
                        switch tt := t.Else.(type) {
                        case *ast.IfStmt:
                                p.Pi("else if(%s) ", expr(tt.Cond))
                                VisitBlockStmt(p, tt.Body)
                        case *ast.BlockStmt:
                                p.Pln("else")
                                VisitBlockStmt(p, tt)
                        }
                }
        case *ast.ForStmt:
                pp := new(Printer)
                VisitStmt(pp, t.Init)
                init := strings.TrimRight(pp.String(), "\n")
                pp.Reset()

                VisitStmt(pp, t.Post)
                post := strings.TrimRight(pp.String(), ";\n")

                p.Pi("for (%s %s; %s) ", init, expr(t.Cond), post)
                VisitBlockStmt(p, t.Body)
        }
}

func VisitBlockStmt(p *Printer, n *ast.BlockStmt) {
        p.P("{\n")
        p.Indent()
        for _, elem := range n.List {
                VisitStmt(p, elem)
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
                rettyp = expr(fun.Results.List[0].Type)
        }

        params := ""
        if fun.Params.NumFields() != 0 {
                paraml := make([]string, 0)
                for _, f := range fun.Params.List {
                        param := field(f)
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
                switch t := d.Type.(type) {
                case *ast.ArrayType:
                        p.Pln("%s %s[%s];", expr(t.Elt), d.Names[0].Name, expr(t.Len))
                case *ast.StarExpr:
                        p.Pln("%s* %s;", expr(t.X), d.Names[0].Name)
                default:
                        p.Pln("%s %s;", expr(d.Type), d.Names[0].Name)
                }
        case *ast.ImportSpec:
                path, _ := strconv.Unquote(d.Path.Value)
                p.Pln(`#include <%s.h>`, path)
        case *ast.TypeSpec:
                switch t := d.Type.(type) {
                case *ast.Ident:
                        p.Pln("typedef %s %s;", t.Name, d.Name)
                case *ast.StructType:
                        p.Pln("struct %s {", d.Name)
                        p.Indent()
                        for _, f := range t.Fields.List {
                                p.Pln("%s;", field(f))
                        }
                        p.Unindent()
                        p.Pln("};")
                }
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
        if *printAST {
                ast.Print(fset, f)
        }
        p := NewPrinter()
        VisitFile(p, f)
        p.WriteTo(os.Stdout)
}
