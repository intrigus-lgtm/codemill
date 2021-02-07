package responsebody

import (
	"sort"
	"strings"

	"github.com/gagliardetto/codebox/scanner"
	"github.com/gagliardetto/codemill/x"
	. "github.com/gagliardetto/cqlgen/jen"
	"github.com/gagliardetto/feparser"
	. "github.com/gagliardetto/utilz"
)

func (han *Handler) GenerateCodeQL(impAdder x.ImportAdder, mdl *x.XModel, rootModuleGroup *Group) error {
	if err := mdl.Validate(); err != nil {
		return err
	}
	if err := han.Validate(mdl); err != nil {
		return err
	}

	{
		// Add imports:
		//impAdder.Import("DataFlow::PathGraph")
	}

	className := mdl.Name
	allPathVersions := func() []string {
		res := make([]string, 0)
		mods := mdl.ListModules()
		for _, mod := range mods {
			res = append(res, mod.PathVersionClean())
		}
		sort.Strings(res)
		return res
	}()

	{
		addedCount := 0
		funcModelsClassName := feparser.NewCodeQlName(className)
		tmp := DoGroup(func(tempFuncsModel *Group) {
			tempFuncsModel.Doc("Models HTTP ResponseBody.")
			tempFuncsModel.Private().Class().Id(funcModelsClassName).Extends().List(
				Id("HTTP::ResponseBody::Range"),
			).BlockFunc(
				func(funcModelsClassGroup *Group) {
					funcModelsClassGroup.String().Id("package").Semicolon().Line()
					funcModelsClassGroup.Id("DataFlow::CallNode").Id("bodySetterCall").Semicolon().Line()
					funcModelsClassGroup.String().Id("contentType").Semicolon().Line()

					funcModelsClassGroup.Id(funcModelsClassName).Call().BlockFunc(
						func(funcModelsSelfMethodGroup *Group) {
							{
								funcModelsSelfMethodGroup.DoGroup(
									func(groupCase *Group) {
										// TODO
										pathCodez := make([]Code, 0)
										for _, pathVersion := range allPathVersions {
											{
												pc := cql_MethodBodyWithCtFromFuncName(mdl, pathVersion, groupCase)
												pathCodez = append(pathCodez, pc...)
											}
											{
												pc := cql_MethodBodyWithCt(mdl, pathVersion, groupCase)
												pathCodez = append(pathCodez, pc...)
											}

											if len(pathCodez) > 0 {
												if addedCount > 0 {
													groupCase.Or()
												}
												path, _ := scanner.SplitPathVersion(pathVersion)
												groupCase.Commentf("HTTP ResponseBody models for package: %s", pathVersion)
												groupCase.Id("package").Eq().Add(x.CqlFormatPackagePath(path)).And()

												groupCase.Parens(
													Join(
														Or(),
														pathCodez...,
													),
												)

												addedCount++
											}
										}
									})
							}
						})

					funcModelsClassGroup.Override().Id("string").Id("getAContentType").Call().BlockFunc(
						func(overrideBlockGroup *Group) {
							overrideBlockGroup.Id("result").Eq().Id("contentType")
						})

					funcModelsClassGroup.Override().Id("HTTP::ResponseWriter").Id("getResponseWriter").Call().BlockFunc(
						func(overrideBlockGroup *Group) {
							overrideBlockGroup.None()
						})
				})
		})
		if addedCount > 0 {
			rootModuleGroup.Add(tmp)
		}
	}

	return nil
}

func GetFunc(qual *x.FuncQualifier) x.FuncInterface {

	source := x.GetCachedSource(qual.Path, qual.Version)
	if source == nil {
		Fatalf("Source not found: %s@%s", qual.Path, qual.Version)
	}
	// Find the func/type-method/interface-method:
	fn := x.FindFuncByID(source, qual.ID)
	if fn == nil {
		Fatalf("Func not found: %q", qual.ID)
	}

	return fn
}

func GetFuncQualifierCodeElements(qual *x.FuncQualifier) (x.FuncInterface, Code) {
	fn := GetFunc(qual)

	parameterIndexes := x.MustPosToRelativeParamIndexes(fn, qual.Pos)
	code := x.GenCqlParamQual("bodySetterCall", "getArgument", fn, parameterIndexes)

	return fn, code
}

func guessContentTypeFromFuncName(name string) string {
	name = strings.ToLower(name)

	if strings.Contains(name, "jsonp") {
		return "application/javascript"
	}
	if strings.Contains(name, "json") {
		return "application/json"
	}
	if strings.Contains(name, "xml") {
		return "text/xml"
	}
	if strings.Contains(name, "yaml") || strings.Contains(name, "yml") {
		return "application/x-yaml"
	}
	if strings.Contains(name, "html") {
		return "text/html"
	}
	if strings.Contains(name, "string") || strings.Contains(name, "text") {
		return "text/plain"
	}
	if strings.Contains(name, "error") {
		// NOTE: this might be not correct.
		return "text/plain"
	}
	return "TODO"
}

// cql_MethodBodyWithCtFromFuncName generates model statements for MethodBodyWithCtFromFuncName
func cql_MethodBodyWithCtFromFuncName(mdl *x.XModel, pathVersion string, groupCase *Group) []Code {

	// Assuming the validation has already been done:
	mtdBodyWithCtFromFuncName := mdl.Methods.ByName(MethodBodyWithCtFromFuncName)
	if len(mtdBodyWithCtFromFuncName.Selectors) == 0 {
		Infof("No selectors found for %q method.", mtdBodyWithCtFromFuncName.Name)
		return nil
	}

	b2fe, b2tm, b2itm, err := x.GroupFuncSelectors(mtdBodyWithCtFromFuncName)
	if err != nil {
		Fatalf("Error while GroupFuncSelectors: %s", err)
	}

	pathCodez := make([]Code, 0)
	// Functions:
	{
		cont, ok := b2fe[pathVersion]
		if ok {
			for _, funcQual := range cont {
				if AllFalse(funcQual.Pos...) {
					continue
				}
				fn := GetFunc(funcQual)
				thing := fn.(*feparser.FEFunc)
				pathCodez = append(pathCodez,
					ParensFunc(
						func(par *Group) {
							par.Commentf("signature: %s", thing.Signature)
							par.Id("bodySetterCall").
								Dot("getTarget").Call().
								Dot("hasQualifiedName").Call(
								Id("package"),
								Lit(thing.Name),
							)

							par.And()

							_, code := GetFuncQualifierCodeElements(funcQual)
							par.Id("this").Eq().Add(code)

							par.And()

							par.Id("contentType").Eq().Lit(guessContentTypeFromFuncName(thing.Name))
						},
					),
				)
			}

		}
	}
	// Type methods:
	{
		cont, ok := b2tm[pathVersion]
		if ok {
			keys := func(v map[string]x.FuncQualifierSlice) []string {
				res := make([]string, 0)
				for key := range v {
					res = append(res, key)
				}
				sort.Strings(res)
				return res
			}(cont)
			for _, receiverTypeID := range keys {
				methodQualifiers := cont[receiverTypeID]
				if len(methodQualifiers) == 0 || !x.HasValidPos(methodQualifiers...) {
					continue
				}
				codez := DoGroup(func(mtdGroup *Group) {
					qual := methodQualifiers[0]
					source := x.GetCachedSource(qual.Path, qual.Version)
					if source == nil {
						Fatalf("Source not found: %s@%s", qual.Path, qual.Version)
					}
					// Find receiver type:
					typ := x.FindTypeByID(source, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}

					mtdGroup.Commentf("Receiver type: %s", typ.TypeString)

					methodIndex := 0
					mtdGroup.ParensFunc(
						func(parMethods *Group) {
							for _, methodQual := range methodQualifiers {
								if AllFalse(methodQual.Pos...) {
									continue
								}
								if methodIndex > 0 {
									parMethods.Or()
								}
								methodIndex++

								fn := GetFunc(methodQual)
								thing := fn.(*feparser.FETypeMethod)

								parMethods.ParensFunc(
									func(par *Group) {
										par.Commentf("signature: %s", thing.Func.Signature)

										par.Id("bodySetterCall").
											Eq().
											Any(
												DoGroup(func(gr *Group) {
													gr.Id("Method").Id("m")
												}),
												DoGroup(func(gr *Group) {
													gr.Id("m").Dot("hasQualifiedName").Call(
														Id("package"),
														Lit(thing.Receiver.TypeName),
														Lit(thing.Func.Name),
													)
												}),
												nil,
											).Dot("getACall").Call()

										par.And()

										_, code := GetFuncQualifierCodeElements(methodQual)
										par.Id("this").Eq().Add(code)

										par.And()

										par.Id("contentType").Eq().Lit(guessContentTypeFromFuncName(thing.Func.Name))
									},
								)

							}
						},
					)

				})
				pathCodez = append(pathCodez, codez)
			}
		}
	}
	// Interface methods:
	{
		contb2itm, ok := b2itm[pathVersion]
		if ok {
			keys := func(v map[string]x.FuncQualifierSlice) []string {
				res := make([]string, 0)
				for key := range v {
					res = append(res, key)
				}
				sort.Strings(res)
				return res
			}(contb2itm)
			for _, receiverTypeID := range keys {
				methodQualifiers := contb2itm[receiverTypeID]
				if len(methodQualifiers) == 0 || !x.HasValidPos(methodQualifiers...) {
					continue
				}
				codez := DoGroup(func(mtdGroup *Group) {
					qual := methodQualifiers[0]
					source := x.GetCachedSource(qual.Path, qual.Version)
					if source == nil {
						Fatalf("Source not found: %s@%s", qual.Path, qual.Version)
					}
					// Find receiver type:
					typ := x.FindTypeByID(source, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}
					mtdGroup.Commentf("Receiver interface: %s", typ.TypeString)

					methodIndex := 0
					mtdGroup.ParensFunc(
						func(parMethods *Group) {
							for _, methodQual := range methodQualifiers {
								if AllFalse(methodQual.Pos...) {
									continue
								}
								if methodIndex > 0 {
									parMethods.Or()
								}
								methodIndex++

								fn := GetFunc(methodQual)
								thing := fn.(*feparser.FEInterfaceMethod)

								parMethods.ParensFunc(
									func(par *Group) {
										par.Commentf("signature: %s", thing.Func.Signature)

										par.Id("bodySetterCall").
											Eq().
											Any(
												DoGroup(func(gr *Group) {
													gr.Id("Method").Id("m")
												}),
												DoGroup(func(gr *Group) {
													gr.Id("m").Dot("implements").Call(
														Id("package"),
														Lit(thing.Receiver.TypeName),
														Lit(thing.Func.Name),
													)
												}),
												nil,
											).Dot("getACall").Call()

										par.And()

										_, code := GetFuncQualifierCodeElements(methodQual)
										par.Id("this").Eq().Add(code)

										par.And()

										par.Id("contentType").Eq().Lit(guessContentTypeFromFuncName(thing.Func.Name))
									},
								)

							}
						},
					)

				})
				pathCodez = append(pathCodez, codez)
			}
		}
	}

	return pathCodez
}

// cql_MethodBodyWithCt generates model statements combining MethodBodyWithCtIsBody and MethodBodyWithCtIsCt.
func cql_MethodBodyWithCt(mdl *x.XModel, pathVersion string, groupCase *Group) []Code {

	mtdBodyWithCtIsBody := mdl.Methods.ByName(MethodBodyWithCtIsBody)
	if len(mtdBodyWithCtIsBody.Selectors) == 0 {
		Infof("No selectors found for %q method.", mtdBodyWithCtIsBody.Name)
		return nil
	}

	b2feBody, b2tmBody, b2itmBody, err := x.GroupFuncSelectors(mtdBodyWithCtIsBody)
	if err != nil {
		Fatalf("Error while GroupFuncSelectors: %s", err)
	}
	//
	mtdBodyWithCtIsCt := mdl.Methods.ByName(MethodBodyWithCtIsCt)
	if len(mtdBodyWithCtIsCt.Selectors) == 0 {
		Infof("No selectors found for %q method.", mtdBodyWithCtIsCt.Name)
		return nil
	}

	b2feCt, b2tmCt, b2itmCt, err := x.GroupFuncSelectors(mtdBodyWithCtIsCt)
	if err != nil {
		Fatalf("Error while GroupFuncSelectors: %s", err)
	}

	pathCodez := make([]Code, 0)
	// Functions:
	{
		cont, ok := b2feBody[pathVersion]
		if ok {
			for _, funcQual := range cont {
				if AllFalse(funcQual.Pos...) {
					continue
				}
				fn := GetFunc(funcQual)
				thing := fn.(*feparser.FEFunc)
				pathCodez = append(pathCodez,
					ParensFunc(
						func(par *Group) {
							par.Commentf("signature: %s", thing.Signature)
							par.Id("bodySetterCall").
								Dot("getTarget").Call().
								Dot("hasQualifiedName").Call(
								Id("package"),
								Lit(thing.Name),
							)

							par.And()

							_, code := GetFuncQualifierCodeElements(funcQual)
							par.Id("this").Eq().Add(code)

							par.And()

							{
								ctQual := b2feCt[pathVersion].ByBasicQualifier(funcQual.BasicQualifier)
								_, code := GetFuncQualifierCodeElements(ctQual)
								par.Id("contentType").Eq().Add(code)
							}
						},
					),
				)
			}

		}
	}
	// Type methods:
	{
		cont, ok := b2tmBody[pathVersion]
		if ok {
			keys := func(v map[string]x.FuncQualifierSlice) []string {
				res := make([]string, 0)
				for key := range v {
					res = append(res, key)
				}
				sort.Strings(res)
				return res
			}(cont)
			for _, receiverTypeID := range keys {
				methodQualifiers := cont[receiverTypeID]
				if len(methodQualifiers) == 0 || !x.HasValidPos(methodQualifiers...) {
					continue
				}
				codez := DoGroup(func(mtdGroup *Group) {
					qual := methodQualifiers[0]
					source := x.GetCachedSource(qual.Path, qual.Version)
					if source == nil {
						Fatalf("Source not found: %s@%s", qual.Path, qual.Version)
					}
					// Find receiver type:
					typ := x.FindTypeByID(source, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}

					mtdGroup.Commentf("Receiver type: %s", typ.TypeString)

					methodIndex := 0
					mtdGroup.ParensFunc(
						func(parMethods *Group) {
							for _, methodQual := range methodQualifiers {
								if AllFalse(methodQual.Pos...) {
									continue
								}
								if methodIndex > 0 {
									parMethods.Or()
								}
								methodIndex++

								fn := GetFunc(methodQual)
								thing := fn.(*feparser.FETypeMethod)

								parMethods.ParensFunc(
									func(par *Group) {
										par.Commentf("signature: %s", thing.Func.Signature)

										par.Id("bodySetterCall").
											Eq().
											Any(
												DoGroup(func(gr *Group) {
													gr.Id("Method").Id("m")
												}),
												DoGroup(func(gr *Group) {
													gr.Id("m").Dot("hasQualifiedName").Call(
														Id("package"),
														Lit(thing.Receiver.TypeName),
														Lit(thing.Func.Name),
													)
												}),
												nil,
											).Dot("getACall").Call()

										par.And()

										_, code := GetFuncQualifierCodeElements(methodQual)
										par.Id("this").Eq().Add(code)

										par.And()

										{
											ctQual := b2tmCt[pathVersion][receiverTypeID].ByBasicQualifier(methodQual.BasicQualifier)
											_, code := GetFuncQualifierCodeElements(ctQual)
											par.Id("contentType").Eq().Add(code)
										}
									},
								)

							}
						},
					)

				})
				pathCodez = append(pathCodez, codez)
			}
		}
	}
	// Interface methods:
	{
		contb2itm, ok := b2itmBody[pathVersion]
		if ok {
			keys := func(v map[string]x.FuncQualifierSlice) []string {
				res := make([]string, 0)
				for key := range v {
					res = append(res, key)
				}
				sort.Strings(res)
				return res
			}(contb2itm)
			for _, receiverTypeID := range keys {
				methodQualifiers := contb2itm[receiverTypeID]
				if len(methodQualifiers) == 0 || !x.HasValidPos(methodQualifiers...) {
					continue
				}
				codez := DoGroup(func(mtdGroup *Group) {
					qual := methodQualifiers[0]
					source := x.GetCachedSource(qual.Path, qual.Version)
					if source == nil {
						Fatalf("Source not found: %s@%s", qual.Path, qual.Version)
					}
					// Find receiver type:
					typ := x.FindTypeByID(source, receiverTypeID)
					if typ == nil {
						Fatalf("Type not found: %q", receiverTypeID)
					}
					mtdGroup.Commentf("Receiver interface: %s", typ.TypeString)

					methodIndex := 0
					mtdGroup.ParensFunc(
						func(parMethods *Group) {
							for _, methodQual := range methodQualifiers {
								if AllFalse(methodQual.Pos...) {
									continue
								}
								if methodIndex > 0 {
									parMethods.Or()
								}
								methodIndex++

								fn := GetFunc(methodQual)
								thing := fn.(*feparser.FEInterfaceMethod)

								parMethods.ParensFunc(
									func(par *Group) {
										par.Commentf("signature: %s", thing.Func.Signature)

										par.Id("bodySetterCall").
											Eq().
											Any(
												DoGroup(func(gr *Group) {
													gr.Id("Method").Id("m")
												}),
												DoGroup(func(gr *Group) {
													gr.Id("m").Dot("implements").Call(
														Id("package"),
														Lit(thing.Receiver.TypeName),
														Lit(thing.Func.Name),
													)
												}),
												nil,
											).Dot("getACall").Call()

										par.And()

										_, code := GetFuncQualifierCodeElements(methodQual)
										par.Id("this").Eq().Add(code)

										par.And()

										{
											ctQual := b2itmCt[pathVersion][receiverTypeID].ByBasicQualifier(methodQual.BasicQualifier)
											_, code := GetFuncQualifierCodeElements(ctQual)
											par.Id("contentType").Eq().Add(code)
										}
									},
								)

							}
						},
					)

				})
				pathCodez = append(pathCodez, codez)
			}
		}
	}

	return pathCodez
}
