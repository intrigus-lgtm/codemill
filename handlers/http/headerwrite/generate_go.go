package headerwrite

import (
	"go/types"
	"os"
	"path/filepath"

	. "github.com/dave/jennifer/jen"
	"github.com/gagliardetto/codebox/gogentools"
	"github.com/gagliardetto/codemill/x"
	"github.com/gagliardetto/feparser"
	. "github.com/gagliardetto/utilz"
)

const (
	// NOTE: hardcoded inside TestQueryContent const.
	InlineExpectationsTestTagHeaderKey = "$headerKey" // Must start with a $ sign.
	InlineExpectationsTestTagHeaderVal = "$headerVal" // Must start with a $ sign.
)

func Tag(keyVarName, valVarName string) Code {
	tg := Sf(
		"%s=%s %s=%s",
		InlineExpectationsTestTagHeaderKey,
		keyVarName,
		InlineExpectationsTestTagHeaderVal,
		valVarName,
	)

	return Comment(tg)
}

const (
	TestQueryContent = `
import go
import TestUtilities.InlineExpectationsTest

class HttpHeaderWriteTest extends InlineExpectationsTest {
  HttpHeaderWriteTest() { this = "HttpHeaderWriteTest" }

  override string getARelevantTag() { result = ["headerKey", "headerVal"] }

  override predicate hasActualResult(string file, int line, string element, string tag, string value) {
    exists(HTTP::HeaderWrite hw |
      hw.hasLocationInfo(file, line, _, _, _) and
      (
        element = hw.getName().toString() and
        value = hw.getName().toString() and
        tag = "headerKey"
        or
        element = hw.getValue().toString() and
        value = hw.getValue().toString() and
        tag = "headerVal"
      )
    )
  }
}
`
)

func NewTestFile(includeBoilerplace bool) *File {
	file := NewFile("main")
	// Set a prefix to avoid collision between variable names and packages:
	file.PackagePrefix = "cql"
	// Add comment to file:
	file.HeaderComment("Code generated by https://github.com/gagliardetto. DO NOT EDIT.")

	if includeBoilerplace {
		{
			// main function:
			file.Func().Id("main").Params().Block()
		}
		{
			// The `source` function returns a new URL:
			code := Func().
				Id("source").
				Params().
				Interface().
				Block(Return(Nil()))
			file.Add(code.Line())
		}
	}
	return file
}

var (
	IncludeCommentsInGeneratedGo bool
)

func (han *Handler) GenerateGo(parentDir string, mdl *x.XModel) error {
	if err := mdl.Validate(); err != nil {
		return err
	}
	if err := han.Validate(mdl); err != nil {
		return err
	}
	// TODO:
	// - Validate Pos.

	// Check if there are multiple versions of a same package:
	mods := mdl.ListModules()
	if x.HasMultiversion(mods) {
		Ln(RedBG("Has multiversion"))
	}
	// If there are no multiple versions of the same module,
	// that means we can save all the code to one file.
	allInOneFile := !x.HasMultiversion(mods)

	// Create the directory for the tests for this model:
	outDir := filepath.Join(parentDir, feparser.NewCodeQlName(mdl.Name))
	MustCreateFolderIfNotExists(outDir, os.ModePerm)

	// Assuming the validation has already been done:
	MethodWriteHeaderKey := mdl.Methods.ByName(MethodWriteHeaderKey)
	if len(MethodWriteHeaderKey.Selectors) == 0 {
		Infof("No selectors found for %q method.", MethodWriteHeaderKey.Name)
		return nil
	}

	MethodWriteHeaderVal := mdl.Methods.ByName(MethodWriteHeaderVal)
	if len(MethodWriteHeaderVal.Selectors) == 0 {
		Infof("No selectors found for %q method.", MethodWriteHeaderVal.Name)
		return nil
	}

	allPathVersions := mdl.ListAllPathVersions()

	file := NewTestFile(true)

	for _, pathVersion := range allPathVersions {
		if !allInOneFile {
			// Reset file:
			file = NewTestFile(true)
		}
		codez := make([]Code, 0)

		_, b2tmKey, b2itmKey, err := x.GroupFuncSelectors(MethodWriteHeaderKey)
		if err != nil {
			Fatalf("Error while GroupFuncSelectors: %s", err)
		}
		_, b2tmVal, b2itmVal, err := x.GroupFuncSelectors(MethodWriteHeaderVal)
		if err != nil {
			Fatalf("Error while GroupFuncSelectors: %s", err)
		}
		// TODO: consider also header writes done with a function?

		{
			codezTypeMethods := make([]Code, 0)
			b2tmKey.IterValid(pathVersion,
				func(receiverTypeID string, methodQualifiers x.FuncQualifierSlice) {

					qual := methodQualifiers[0]
					// Find receiver type:
					typ := x.FindType(qual.Path, qual.Version, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}

					gogentools.ImportPackage(file, typ.PkgPath, typ.PkgName)

					code := BlockFunc(
						func(groupCase *Group) {

							for _, keyMethodQual := range methodQualifiers {
								fn := x.GetFuncByQualifier(keyMethodQual)
								thing := fn.(*feparser.FETypeMethod)
								x.AddImportsFromFunc(file, fn)

								// TODO:
								// - Check if found.
								valMethodQual := b2tmVal[pathVersion][receiverTypeID].ByBasicQualifier(keyMethodQual.BasicQualifier)

								{
									if AllFalse(keyMethodQual.Pos...) {
										continue
									}
									groupCase.Comment(thing.Func.Signature)

									blocksOfCases := generateGoTestBlock_Method(
										file,
										thing,
										keyMethodQual,
										valMethodQual,
									)
									if len(blocksOfCases) == 1 {
										groupCase.Add(blocksOfCases...)
									} else {
										groupCase.Block(blocksOfCases...)
									}
								}

							}
						})
					// TODO: what if no flows are enabled? Check that before adding the comment.
					codezTypeMethods = append(codezTypeMethods,
						Commentf("Header write via method calls on %s.", typ.QualifiedName).
							Line().
							Add(code),
					)
				})
			if len(codezTypeMethods) > 0 {
				codez = append(codez,
					Comment("Header write via method calls.").
						Line().
						Block(codezTypeMethods...),
				)
			}
		}

		{
			codezIfaceMethods := make([]Code, 0)
			b2itmKey.IterValid(pathVersion,
				func(receiverTypeID string, methodQualifiers x.FuncQualifierSlice) {

					qual := methodQualifiers[0]
					// Find receiver type:
					typ := x.FindType(qual.Path, qual.Version, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}

					gogentools.ImportPackage(file, typ.PkgPath, typ.PkgName)

					code := BlockFunc(
						func(groupCase *Group) {

							for _, keyMethodQual := range methodQualifiers {
								fn := x.GetFuncByQualifier(keyMethodQual)
								thing := fn.(*feparser.FEInterfaceMethod)
								x.AddImportsFromFunc(file, fn)

								// TODO:
								// - Check if found.
								valMethodQual := b2itmVal[pathVersion][receiverTypeID].ByBasicQualifier(keyMethodQual.BasicQualifier)

								{
									if AllFalse(keyMethodQual.Pos...) {
										continue
									}
									groupCase.Comment(thing.Func.Signature)

									converted := feparser.FEIToFET(thing)
									blocksOfCases := generateGoTestBlock_Method(
										file,
										converted,
										keyMethodQual,
										valMethodQual,
									)
									if len(blocksOfCases) == 1 {
										groupCase.Add(blocksOfCases...)
									} else {
										groupCase.Block(blocksOfCases...)
									}
								}
							}
						})
					codezIfaceMethods = append(codezIfaceMethods,
						Commentf("Header write via method calls on %s interface.", typ.QualifiedName).
							Line().
							Add(code),
					)
				})

			if len(codezIfaceMethods) > 0 {
				codez = append(codez,
					Comment("Header write via interface method calls.").
						Line().
						Block(codezIfaceMethods...),
				)
			}
		}

		{
			file.Commentf("Package %s", pathVersion)
			file.Func().Id(feparser.FormatCodeQlName(pathVersion)).Params().Block(codez...)
		}

		if !allInOneFile {
			file.PackageComment("//go:generate depstubber --vendor --auto")

			pkgDstDirpath := filepath.Join(outDir, feparser.FormatID("Model", mdl.Name, "For", feparser.FormatCodeQlName(pathVersion)))
			MustCreateFolderIfNotExists(pkgDstDirpath, os.ModePerm)

			assetFileName := feparser.FormatID("Model", mdl.Name, "For", feparser.FormatCodeQlName(pathVersion)) + ".go"
			if err := x.SaveGoFile(pkgDstDirpath, assetFileName, file); err != nil {
				Fatalf("Error while saving go file: %s", err)
			}

			if err := x.WriteGoModFile(pkgDstDirpath, pathVersion); err != nil {
				Fatalf("Error while saving go.mod file: %s", err)
			}
			if err := x.WriteCodeQLTestQuery(pkgDstDirpath, x.DefaultCodeQLTestFileName, TestQueryContent); err != nil {
				Fatalf("Error while saving <name>.ql file: %s", err)
			}
			if err := x.WriteEmptyCodeQLDotExpectedFile(pkgDstDirpath, x.DefaultCodeQLTestFileName); err != nil {
				Fatalf("Error while saving <name>.expected file: %s", err)
			}
		}
	}

	if allInOneFile {
		file.PackageComment("//go:generate depstubber --vendor --auto")

		pkgDstDirpath := outDir
		MustCreateFolderIfNotExists(pkgDstDirpath, os.ModePerm)

		assetFileName := feparser.FormatID("Model", mdl.Name) + ".go"
		if err := x.SaveGoFile(pkgDstDirpath, assetFileName, file); err != nil {
			Fatalf("Error while saving go file: %s", err)
		}

		if err := x.WriteGoModFile(pkgDstDirpath, allPathVersions...); err != nil {
			Fatalf("Error while saving go.mod file: %s", err)
		}
		if err := x.WriteCodeQLTestQuery(pkgDstDirpath, x.DefaultCodeQLTestFileName, TestQueryContent); err != nil {
			Fatalf("Error while saving <name>.ql file: %s", err)
		}
		if err := x.WriteEmptyCodeQLDotExpectedFile(pkgDstDirpath, x.DefaultCodeQLTestFileName); err != nil {
			Fatalf("Error while saving <name>.expected file: %s", err)
		}
	}
	return nil
}

// Comments adds comments to a Group (if enabled), and returns the group.
func Comments(group *Group, comments ...string) *Group {
	if IncludeCommentsInGeneratedGo {
		for _, comment := range comments {
			group.Line().Comment(comment)
		}
	}
	return group
}

func newStatement() *Statement {
	return &Statement{}
}

func generateGoTestBlock_Method(
	file *File,
	fe *feparser.FETypeMethod,
	qualHeaderKey *x.FuncQualifier,
	qualHeaderVal *x.FuncQualifier,
) []Code {
	childBlocks := make([]Code, 0)

	headerKeyIndexes := x.MustPosToRelativeParamIndexes(fe, qualHeaderKey.Pos)
	if len(headerKeyIndexes) != 1 {
		Fatalf("headerKeyIndexes len is not 1: %v", qualHeaderKey)
	}
	headerValIndexes := x.MustPosToRelativeParamIndexes(fe, qualHeaderVal.Pos)
	if len(headerValIndexes) != 1 {
		Fatalf("headerValIndexes len is not 1: %v", qualHeaderVal)
	}

	childBlock := generate_Method(
		file,
		fe,
		headerKeyIndexes[0],
		headerValIndexes[0],
	)
	{
		if childBlock != nil {
			childBlocks = append(childBlocks, childBlock)
		} else {
			Warnf(Sf("NOTHING GENERATED; qualHeaderKey %v, qualHeaderVal %v", qualHeaderKey.Pos, qualHeaderVal.Pos))
		}
	}

	return childBlocks
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func generate_Method(file *File, fe *feparser.FETypeMethod, indexKey int, indexVal int) *Statement {

	keyParam := fe.Func.Parameters[indexKey]
	keyParam.VarName = gogentools.NewNameWithPrefix(feparser.NewLowerTitleName("key", keyParam.TypeName))

	valParam := fe.Func.Parameters[indexVal]
	valParam.VarName = gogentools.NewNameWithPrefix(feparser.NewLowerTitleName("val", valParam.TypeName))

	code := BlockFunc(
		func(groupCase *Group) {

			ComposeTypeAssertion(file, groupCase, keyParam.VarName, keyParam.GetOriginal().GetType(), keyParam.GetOriginal().IsVariadic())
			ComposeTypeAssertion(file, groupCase, valParam.VarName, valParam.GetOriginal().GetType(), valParam.GetOriginal().IsVariadic())

			Comments(groupCase, "Declare medium object/interface:")
			groupCase.Var().Id("rece").Qual(fe.Receiver.PkgPath, fe.Receiver.TypeName)

			gogentools.ImportPackage(file, fe.Func.PkgPath, fe.Func.PkgName)

			groupCase.Id("rece").Dot(fe.Func.Name).CallFunc(
				func(call *Group) {

					tpFun := fe.Func.GetOriginal().GetType().(*types.Signature)

					zeroVals := gogentools.ScanTupleOfZeroValues(file, tpFun.Params(), fe.Func.GetOriginal().IsVariadic())

					for i, zero := range zeroVals {
						isConsidered := IntSliceContains([]int{indexKey, indexVal}, i)
						if isConsidered {
							call.Id(fe.Func.Parameters[i].VarName)
						} else {
							call.Add(zero)
						}
					}

				},
			).Add(Tag(keyParam.VarName, valParam.VarName))

		})
	return code
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// declare `name := source(1).(Type)`
func ComposeTypeAssertion(file *File, group *Group, varName string, typ types.Type, isVariadic bool) {
	assertContent := newStatement()
	if isVariadic {
		if slice, ok := typ.(*types.Slice); ok {
			gogentools.ComposeTypeDeclaration(file, assertContent, slice.Elem())
		} else {
			gogentools.ComposeTypeDeclaration(file, assertContent, typ)
		}
	} else {
		gogentools.ComposeTypeDeclaration(file, assertContent, typ)
	}
	group.Id(varName).Op(":=").Id("source").Call().Assert(assertContent)
}
