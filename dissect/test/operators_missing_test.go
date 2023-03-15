package test

import (
	"strings"
	"testing"

	"github.com/redraskal/r6-dissect/dissect/ubi"
)

func Test_operatorsMissing(tt *testing.T) {
	ourOpNames, ubiOpNames := assembleOperatorNames(tt)

	opsMissingInOur := sliceDiff(ubiOpNames, ourOpNames)
	if len(opsMissingInOur) > 0 {
		tt.Errorf("operators missing in source files:")
		for _, n := range opsMissingInOur {
			tt.Errorf(`> "%s"`, n)
		}
	}
}

func Test_operatorsRedundant(tt *testing.T) {
	ourOpNames, ubiOpNames := assembleOperatorNames(tt)

	opsMissingInUbi := sliceDiff(ourOpNames, ubiOpNames)
	if len(opsMissingInUbi) > 0 {
		tt.Errorf("operators in our source files that Ubisoft does not provide:")
		for _, n := range opsMissingInUbi {
			tt.Errorf(`> "%s"`, n)
		}
	}
}

func assembleOperatorNames(tt *testing.T) (us []string, ubisoft []string) {
	pkg, err := loadPackage()
	if err != nil {
		tt.Fatal(err)
	}
	operatorConsts, err := getOperatorDefs(pkg)
	if err != nil {
		tt.Fatalf("could not determine operator consts: %v", err)
	}
	ourOpNames := make([]string, len(operatorConsts))
	for i, c := range operatorConsts {
		ourOpNames[i] = strings.ToLower(c.Name())
	}

	ubiOpsMap, err := ubi.GetOperatorMap()
	if err != nil {
		tt.Fatalf("could not get operators from Ubisoft")
	}
	ubiOpNames := make([]string, len(ubiOpsMap))
	i := 0
	for n := range ubiOpsMap {
		ubiOpNames[i] = strings.ToLower(n)
		i++
	}
	return ourOpNames, ubiOpNames
}
