// Usage is analogue to the official stringer (go/x/tools/stringer), except that
// -type cannot be a list, but needs to be a single type.
//
// go:generate go run genops.go -type=<operator_type_name>
//
//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/redraskal/r6-dissect/dissect/ubi"
	"golang.org/x/tools/go/packages"
)

const (
	flagOperatorTypeName string = "type"
	flagAtkValueName     string = "atkval"
	flagDefValueName     string = "defval"
)

// main (and most of this file) is heavily inspired by go/x/tools/stringer
func main() {
	log.SetFlags(0)
	log.SetPrefix("genops: ")

	typeName := flag.String(flagOperatorTypeName, "", "operator type name; required")
	atkValueName := flag.String(flagAtkValueName, "", "value name for attack role; required")
	defValueName := flag.String(flagDefValueName, "", "value name for defense role; required")
	flag.Parse()

	// validate flags are set
	if len(*typeName) == 0 || len(*atkValueName) == 0 || len(*defValueName) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	// assemble full path to invoking file
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	srcFile := path.Join(cwd, os.Getenv("GOFILE"))

	g := Generator{
		operatorTypeName: *typeName,
		roleValueAtk:     *atkValueName,
		roleValueDef:     *defValueName,
	}

	// load input package
	g.parseSrcFile(srcFile)

	g.generate()

	// Format the output.
	src := g.format()

	// Write to file.
	baseName := fmt.Sprintf("%s_roles.go", *typeName)
	outFile := filepath.Join(cwd, strings.ToLower(baseName))
	if err = ioutil.WriteFile(outFile, src, 0644); err != nil {
		log.Fatalf("writing output: %s", err)
	}
}

type Generator struct {
	buf              bytes.Buffer
	operatorTypeName string
	roleTypeName     string
	roleValueAtk     string
	roleValueDef     string
	pkgName          string
	operatorConsts   []*types.Const
}

// parseSrcFile reads an input file and parses its package
// to find the types and values we need for generating our output
func (g *Generator) parseSrcFile(file string) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, file)
	if err != nil {
		log.Fatal(err)
	}
	// validate we have exactly *one* package
	// (since we only accept exactly one type, we should also only get one package)
	if len(pkgs) == 0 {
		log.Fatalf("error: no packages found in %s", file)
	} else if len(pkgs) > 1 {
		log.Fatalf("error: expected exactly 1 package, got %d", len(pkgs))
	}
	g.loadPackage(pkgs[0])
}

// loadPackage processes the provided pkg and attempts to find consts
// with the configured type to obtain a list of operator names
func (g *Generator) loadPackage(pkg *packages.Package) {
	g.pkgName = pkg.Name
	operatorConsts := make([]*types.Const, 0, 100)

	// find all constants that have our target type
	scope := pkg.Types.Scope()
	targetType := scope.Lookup(g.operatorTypeName).Type()
	for _, obj := range pkg.TypesInfo.Defs {
		// assert that types matches
		if obj != nil && obj.Type() == targetType {
			// assert that value is const
			if v, ok := obj.(*types.Const); ok {
				operatorConsts = append(operatorConsts, v)
			}
		}
	}

	// validate we actually found consts with desired type
	if len(operatorConsts) == 0 {
		log.Fatalf("error: did not find any constants of type \"%s\" in package %s", g.operatorTypeName, pkg.Name)
	}
	g.operatorConsts = operatorConsts
	g.roleTypeName = g.getRoleTypeName(pkg)
}

// getRoleTypeName attempts to find the type name for the attack/defense
// values provided via flags
func (g *Generator) getRoleTypeName(pkg *packages.Package) string {
	scope := pkg.Types.Scope()
	// lookup objects for atk/def
	atkType := scope.Lookup(g.roleValueAtk)
	if atkType == nil {
		log.Fatalf(`error: could not resolve -%s value in package "%s"`, flagAtkValueName, pkg.Name)
	}
	defType := scope.Lookup(g.roleValueDef)
	if defType == nil {
		log.Fatalf(`error: could not resolve -%s value in package "%s"`, flagDefValueName, pkg.Name)
	}
	// assert they are the same type
	if atkType.Type() != defType.Type() {
		log.Fatalf(
			`-%s and -%s values need to be same type, got "%s %s" and "%s %s"`,
			flagAtkValueName,
			flagDefValueName,
			g.roleValueAtk,
			atkType,
			g.roleValueDef,
			defType,
		)
	}
	// attempt to case to get .Name() function
	t, ok := atkType.Type().(*types.Named)
	if !ok {
		log.Fatalf(`error: could not cast type of "%s" to types.Named (is %T)`, g.roleValueAtk, atkType.Type())
	}
	return t.Obj().Name()
}

// generate will use everything we compiled previously to write to
// our output file
func (g *Generator) generate() {
	g.printHeader()

	log.Println("retrieving operator data from Ubisoft")
	ops, err := ubi.GetOperatorMap()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("creating operator role map")
	g.printf("var _operatorRoles = map[%s]%s{\n", g.operatorTypeName, g.roleTypeName)
	for _, c := range g.operatorConsts {
		constNameLower := strings.ToLower(c.Name())
		op, exists := ops[constNameLower]
		if !exists {
			log.Printf("WARNING: operator const \"%s\" not present in Ubisoft data\n", c.Name())
			log.Println("         either add it manually or check the const name")
		}

		var roleVal string
		if op.IsAttacker {
			roleVal = g.roleValueAtk
		} else {
			roleVal = g.roleValueDef
		}
		g.printf("%s: %s,\n", c.Val().ExactString(), roleVal)
	}
	g.printf("}\n")

	g.printGetter()
}

func (g *Generator) printHeader() {
	g.printf("// Code generated by \"genops.go %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " "))
	g.printf("package %s\n", g.pkgName)
	g.printf("\n")
	g.printf("import \"errors\"\n")
}

func (g *Generator) printGetter() {
	g.printf("func (i Operator) Role() (%s, error) {\n", g.roleTypeName)
	g.printf("if r, ok := _operatorRoles[i]; ok {\n")
	g.printf("return r, nil\n")
	g.printf("}\n")
	g.printf(`return %s, errors.New("role unknown for this operator")`, g.roleValueAtk)
	g.printf("}\n")
}

func (g *Generator) printf(format string, args ...any) {
	fmt.Fprintf(&g.buf, format, args...)
}

// format returns the gofmt-ed contents of the Generator's buffer.
func (g *Generator) format() []byte {
	src, err := format.Source(g.buf.Bytes())
	if err != nil {
		// Should never happen, but can arise when developing this code.
		// The user can compile the output to see the error.
		log.Printf("warning: internal error: invalid Go generated: %s", err)
		log.Printf("warning: compile the package to analyze the error")
		return g.buf.Bytes()
	}
	return src
}
