package test

import (
	"errors"
	"fmt"
	"go/types"
	"strconv"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
	"golang.org/x/tools/go/packages"
)

func Test_operatorRolesDefined(tt *testing.T) {
	// gather data we need
	pkg, err := loadPackage()
	if err != nil {
		tt.Fatal(err)
	}
	operatorConsts, err := getOperatorDefs(pkg)
	if err != nil {
		tt.Fatalf("could not determine operator consts: %v", err)
	}

	// actual testing
	for _, opDef := range operatorConsts {
		opName := opDef.Name()
		tt.Run(opName, func(t *testing.T) {
			convertedOp, err := parseOpConst(opDef)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				// recover panic, r is nil if haven't panicked
				if r := recover(); r != nil {
					t.Fatalf(`could not determine role for "%s": %v`, opName, r)
				}
			}()
			// should panic if role undefined for operator
			_ = convertedOp.Role()
		})
	}
}

func parseOpConst(c *types.Const) (op dissect.Operator, err error) {
	// validate const has expected type
	underlying := c.Type().Underlying()
	basicType, ok := underlying.(*types.Basic)
	if !ok {
		err = fmt.Errorf(`could not determine basic type for const "%s"`, c.Name())
		return
	}
	// assert that we have uint64
	//
	// this check could also be performed when loading the consts from the package,
	// but I want to keep the check for uint64 and the actual conversion to uint64 in the same function
	if !(basicType.Kind() == types.Uint64) {
		err = fmt.Errorf(`test expects const "%s" to have underlying type uint64, please amend this test accordingly`, c.Name())
		return
	}

	v, err := strconv.ParseUint(c.Val().String(), 10, 64)
	if err != nil {
		err = fmt.Errorf(`could not convert "%s" to uint64`, c.Name())
		return
	}
	return dissect.Operator(v), nil
}

const (
	packageName      string = "github.com/redraskal/r6-dissect/dissect"
	operatorTypeName string = "Operator"
)

func loadPackage() (*packages.Package, error) {
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedTypesInfo}
	pkgs, err := packages.Load(cfg, "pattern="+packageName)
	if err != nil {
		return nil, err
	}

	if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("got error loading package")
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf(`found no package "%s"`, packageName)
	}
	return pkgs[0], nil
}

// getOperatorDefs parses the dissect package and returns all consts with the operator type.
//
// Ideally, during testing, we would of course import the dissect package directly instead
// of parsing it. Since we don't keep a list of operators (and that's probably a good idea from a maintainability side), we
// will have to assemble it dynamically in this function.
func getOperatorDefs(pkg *packages.Package) ([]*types.Const, error) {
	scope := pkg.Types.Scope()
	operatorTypeObj := scope.Lookup(operatorTypeName)
	if operatorTypeObj == nil {
		return nil, fmt.Errorf(`could not lookup type "%s"`, operatorTypeName)
	}
	operatorType := operatorTypeObj.Type()

	operatorDefs := make([]*types.Const, 0, 100)
	objDefs := pkg.TypesInfo.Defs
	for _, obj := range objDefs {
		if obj != nil && types.Identical(obj.Type(), operatorType) {
			if c, ok := obj.(*types.Const); ok {
				operatorDefs = append(operatorDefs, c)
			}
		}
	}

	if len(operatorDefs) == 0 {
		return nil, fmt.Errorf(`found no consts of type "%s" in package "%s"`, operatorTypeName, packageName)
	}
	return operatorDefs, nil
}
