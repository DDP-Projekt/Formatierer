package formatierer

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/DDP-Projekt/Kompilierer/src/ast"
	"github.com/DDP-Projekt/Kompilierer/src/ddptypes"
	"github.com/DDP-Projekt/Kompilierer/src/token"
)

type FormattingOptions struct {
	InsertSpaces bool
}

type formattingVisitor struct {
	document        string
	opts            FormattingOptions
	out             strings.Builder
	indent          int
	currentPos      token.Position
	nextLetterLower bool
}

func (*formattingVisitor) Visitor() {}

func (v *formattingVisitor) write(s string) {
	if v.nextLetterLower {
		v.out.WriteRune(unicode.ToLower(rune(s[0])))
		v.out.WriteString(s[1:])
		v.nextLetterLower = false
		return
	}
	v.out.WriteString(s)
}

func (v *formattingVisitor) writeIndent() {
	for range v.indent {
		if v.opts.InsertSpaces == true {
			v.out.WriteString("    ")
		} else {
			v.out.WriteString("\t")
		}
	}
}

func (v *formattingVisitor) space() {
	v.out.WriteRune(' ')
}

func (v *formattingVisitor) advanceTo(pos token.Position) {
	v.currentPos = pos
	//log.Infof("%v", v.getRangeText(token.Range{Start: token.Position{0, 0}, End: pos}))
}

// writes a newline followed by the current indentation
func (v *formattingVisitor) newline() {
	v.out.WriteRune('\n')
	v.writeIndent()
}

func nominativDefiniteArticle(gender ddptypes.GrammaticalGender) string {
	switch gender {
	case ddptypes.MASKULIN:
		return "Der"
	case ddptypes.FEMININ:
		return "Die"
	default:
		return "Das"
	}
}

func dativDefiniteArticle(gender ddptypes.GrammaticalGender) string {
	if gender == ddptypes.FEMININ {
		return "der"
	}
	return "dem"
}

func nominativIndefiniteArticle(gender ddptypes.GrammaticalGender) string {
	if gender == ddptypes.FEMININ {
		return "eine"
	}
	return "ein"
}

func akkusativIndefiniteArticle(gender ddptypes.GrammaticalGender) string {
	switch gender {
	case ddptypes.MASKULIN:
		return "einen"
	case ddptypes.FEMININ:
		return "eine"
	default:
		return "ein"
	}
}

func dativIndefiniteArticle(gender ddptypes.GrammaticalGender) string {
	if gender == ddptypes.FEMININ {
		return "einer"
	}
	return "einem"
}

func forPronoun(gender ddptypes.GrammaticalGender) string {
	switch gender {
	case ddptypes.MASKULIN:
		return "jeden"
	case ddptypes.FEMININ:
		return "jede"
	default:
		return "jedes"
	}
}

func getTypeName(t ddptypes.Type) string {
	return t.String()
}

// writes the tokens of a function/struct alias, substituting ALIAS_PARAMETER
// tokens with the corresponding argument from args
func (v *formattingVisitor) writeAliasCall(tokens []token.Token, args map[string]ast.Expression) {
	for i, tok := range tokens {
		if tok.Type == token.EOF {
			continue
		}
		if i > 0 {
			diff := tok.Range.Start.Column - tokens[i-1].Range.End.Column
			for range diff {
				v.space()
			}
		}

		if tok.Type == token.ALIAS_PARAMETER {
			name := strings.Trim(tok.Literal, "<>")
			if arg, ok := args[name]; ok && arg != nil {
				switch arg.(type) {
				case *ast.CastAssigneable:
					v.out.WriteRune('(')
					arg.Accept(v)
					v.out.WriteRune(')')
				default:
					arg.Accept(v)
				}
				continue
			}
		}
		v.write(tok.Literal)
	}
}

func (v *formattingVisitor) VisitBadDecl(decl *ast.BadDecl) ast.VisitResult {
	v.write(decl.Tok.Literal)
	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitConstDecl(decl *ast.ConstDecl) ast.VisitResult {
	v.write("Die")
	v.space()
	if decl.IsPublic {
		v.write("öffentliche")
		v.space()
	}
	v.write("Konstante")
	v.space()
	v.write(decl.Name())
	v.space()
	v.write("ist")
	v.space()
	decl.Val.Accept(v)
	v.write(".")

	v.advanceTo(decl.Val.GetRange().End)
	return ast.VisitRecurse
}

func isBoolOp(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Operator {
		case ast.BIN_AND, ast.BIN_UNEQUAL, ast.BIN_LESS, ast.BIN_LESS_EQ, ast.BIN_GREATER, ast.BIN_GREATER_EQ:
			return true
		}
	case *ast.TernaryExpr:
		if e.Operator == ast.TER_BETWEEN {
			return true
		}
	}
	return false
}

func (v *formattingVisitor) VisitVarDecl(decl *ast.VarDecl) ast.VisitResult {
	v.write(nominativDefiniteArticle(decl.Type.Gender()))
	v.space()
	if decl.IsPublic {
		v.write("öffentliche")
		v.space()
	}
	if decl.IsExternVisible {
		v.write("extern sichtbare")
		v.space()
	}

	v.write(getTypeName(decl.Type))
	v.space()
	v.write(decl.Name())
	v.space()
	v.write("ist")
	v.space()

	if decl.Type == ddptypes.WAHRHEITSWERT {
		if unary, isUnary := decl.InitVal.(*ast.UnaryExpr); isUnary && unary.Operator == ast.UN_NOT && isBoolOp(unary.Rhs) {
			v.write("falsch, wenn")
			v.space()
			unary.Rhs.Accept(v)
		} else if isBoolOp(decl.InitVal) {
			v.write("wahr, wenn")
			v.space()
			decl.InitVal.Accept(v)
		} else {
			decl.InitVal.Accept(v)
		}
	} else {
		decl.InitVal.Accept(v)
	}

	v.write(".")

	v.advanceTo(decl.InitVal.GetRange().End)

	return ast.VisitRecurse
}

func (v *formattingVisitor) writeParameterList(params []ast.ParameterInfo) {
	if len(params) == 0 {
		return
	}

	if len(params) == 1 {
		v.write("mit dem Parameter")
		v.space()
		v.write(params[0].Name.Literal)
		v.space()
		v.write("vom Typ")
		v.space()
		v.write(params[0].Type.String())
	} else {
		v.write("mit den Parametern")
		v.space()
		for i, param := range params {
			if i > 0 {
				if i == len(params)-1 {
					v.space()
					v.write("und")
					v.space()
				} else {
					v.write(", ")
				}
			}
			v.write(param.Name.Literal)
		}
		v.space()
		v.write("vom Typ")
		v.space()
		for i, param := range params {
			if i > 0 {
				if i == len(params)-1 {
					v.space()
					v.write("und")
					v.space()
				} else {
					v.write(", ")
				}
			}
			v.write(param.Type.String())
		}
	}
	v.write(", ")
}

// writes the ""Alias1" oder\n\t"Alias2"" part
// shared by function and struct declarations
func (v *formattingVisitor) writeAliasList(aliases []string) {
	v.indent++
	for i, alias := range aliases {
		v.newline()
		v.write(alias)
		if i < len(aliases)-1 {
			if i == len(aliases)-2 {
				v.space()
				v.write("oder")
			} else {
				v.write(",")
			}
		}
	}
	v.indent--
}

func (v *formattingVisitor) VisitFuncDecl(decl *ast.FuncDecl) ast.VisitResult {
	v.write("Die")
	v.space()
	if decl.IsPublic {
		v.write("öffentliche")
		v.space()
	}
	if ast.IsGeneric(decl) {
		v.write("generische")
		v.space()
	}
	v.write("Funktion")
	v.space()
	v.write(decl.Name())
	v.space()
	v.writeParameterList(decl.Parameters)

	v.write("gibt")
	v.space()

	if ddptypes.IsVoid(decl.ReturnType) {
		v.write("nichts")
	} else if _, isGeneric := decl.ReturnType.(ddptypes.GenericType); isGeneric {
		// TODO: fix in Kompilierer
		retText := v.getRangeText(decl.ReturnTypeRange)
		startIdx := strings.Index(retText, "gibt ") + 5
		endIdx := strings.LastIndex(retText, decl.ReturnType.String()) - 1
		v.write(retText[startIdx:endIdx])
		v.space()
		v.write(decl.ReturnType.String())
	} else {
		v.write(akkusativIndefiniteArticle(decl.ReturnType.Gender()))
		v.space()
		v.write(decl.ReturnType.String())
	}
	v.space()
	v.write("zurück")

	if decl.IsExternVisible {
		v.write(", ist extern sichtbar")
	}

	switch {
	case ast.IsExternFunc(decl):
		v.write(",\nist in")
		v.space()
		v.write(decl.ExternFile.Literal)
		v.write(" definiert")
	case ast.IsForwardDecl(decl):
		v.write(",")
		v.newline()
		v.write("wird später definiert")
	case ast.IsGeneric(decl):
		v.write(", macht:")
		text := v.getRangeText(decl.GetRange()) // TODO: somehow fix in Kompilierer
		startIdx := strings.Index(text, ":") + 1
		endIdx := strings.Index(text, "\nUnd kann so benutzt werden:")
		v.write(text[startIdx:endIdx])
	case decl.Body != nil:
		v.write(", macht")
		decl.Body.Accept(v)
	}

	if ast.IsOperatorOverload(decl) {
		v.write("\n")
		v.write("Und überlädt den")
		v.space()
		v.write(strconv.Quote(decl.Operator.String()))
		v.space()
		v.write("Operator.")
	} else {
		aliases := make([]string, 0, len(decl.Aliases))
		for _, alias := range decl.Aliases {
			if alias.Negated {
				aliases = append(aliases, alias.GetOriginal().Literal)
				break
			}
			aliases = append(aliases, alias.GetOriginal().Literal)
		}
		v.write("\n")
		v.write("Und kann so benutzt werden:")
		if len(aliases) != 0 {
			v.writeAliasList(aliases)
		}
	}

	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitFuncDef(decl *ast.FuncDef) ast.VisitResult {
	v.write("Die Funktion")
	v.space()
	v.write(decl.Func.Name())
	v.space()
	v.write("macht")
	decl.Body.Accept(v)
	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitStructDecl(decl *ast.StructDecl) ast.VisitResult {
	v.write("Wir nennen die")
	v.space()
	if decl.IsPublic {
		v.write("öffentliche")
		v.space()
	}
	if ast.IsGeneric(decl) {
		v.write("generische")
		v.space()
	}

	v.write("Kombination aus")
	v.indent++
	for _, field := range decl.Fields {
		v.newline()
		if fieldDecl, ok := field.(*ast.VarDecl); ok {
			v.write(dativDefiniteArticle(fieldDecl.Type.Gender()))
			v.space()
			if fieldDecl.IsPublic {
				v.write("öffentlichen")
				v.space()
			}
			v.write(fieldDecl.Type.String())
			v.space()
			v.write(fieldDecl.Name())
			if fieldDecl.InitVal != nil {
				v.space()
				v.write("mit Standardwert")
				v.space()
				fieldDecl.InitVal.Accept(v)
			}
		}
		v.write(",")
	}
	v.indent--
	v.newline()
	v.write(akkusativIndefiniteArticle(decl.Type.Gender()))
	v.space()
	v.write(decl.Name())
	v.write(", und erstellen sie so:")

	aliases := make([]string, 0, len(decl.Aliases))
	for _, alias := range decl.Aliases {
		aliases = append(aliases, alias.GetOriginal().Literal)
	}
	if len(aliases) != 0 {
		v.writeAliasList(aliases)
	}

	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitTypeAliasDecl(decl *ast.TypeAliasDecl) ast.VisitResult {
	v.write("Wir nennen")
	v.space()
	v.write(akkusativIndefiniteArticle(decl.Underlying.Gender()))
	v.space()
	v.write(decl.Underlying.String())
	v.space()
	if decl.IsPublic {
		v.write("öffentlich")
		v.space()
	}

	v.write("auch")
	v.space()
	v.write(akkusativIndefiniteArticle(decl.Type.Gender()))
	v.space()
	v.write(decl.Name())
	v.write(".")

	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitTypeDefDecl(decl *ast.TypeDefDecl) ast.VisitResult {
	v.write("Wir definieren")
	v.space()
	v.write(akkusativIndefiniteArticle(decl.Type.Gender()))
	v.space()
	v.write(decl.Name())
	v.space()
	if decl.IsPublic {
		v.write("öffentlich")
		v.space()
	}
	v.write("als")
	v.space()
	v.write(akkusativIndefiniteArticle(decl.Underlying.Gender()))
	v.space()
	v.write(decl.Underlying.String())
	v.write(".")

	v.advanceTo(decl.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBadExpr(expr *ast.BadExpr) ast.VisitResult {
	v.write(expr.Tok.Literal)
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitIdent(expr *ast.Ident) ast.VisitResult {
	v.write(expr.Literal.Literal)
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitIndexing(expr *ast.Indexing) ast.VisitResult {
	expr.Lhs.Accept(v)
	v.space()
	v.write("an der Stelle")
	v.space()
	expr.Index.Accept(v)
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitFieldAccess(expr *ast.FieldAccess) ast.VisitResult {
	v.write(expr.Field.Literal.Literal)
	v.space()
	v.write("von")
	v.space()
	expr.Rhs.Accept(v)
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitIntLit(expr *ast.IntLit) ast.VisitResult {
	v.write(strconv.FormatInt(expr.Value, 10))
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitFloatLit(expr *ast.FloatLit) ast.VisitResult {
	str := strconv.FormatFloat(expr.Value, 'f', -1, 64)
	if !strings.Contains(str, ".") {
		str += ".0"
	}
	v.write(strings.Replace(str, ".", ",", 1))
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBoolLit(expr *ast.BoolLit) ast.VisitResult {
	if expr.Value {
		v.write("wahr")
	} else {
		v.write("falsch")
	}
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitCharLit(expr *ast.CharLit) ast.VisitResult {
	v.write(strconv.QuoteRune(expr.Value))
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitStringLit(expr *ast.StringLit) ast.VisitResult {
	v.write(strconv.Quote(expr.Value))
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitListLit(expr *ast.ListLit) ast.VisitResult {
	if expr.Values != nil {
		v.write("eine Liste, die aus")
		v.space()
		for i, val := range expr.Values {
			if i > 0 {
				v.write(", ")
			}
			val.Accept(v)
		}
		v.space()
		v.write("besteht")
	} else if expr.Value != nil && expr.Count != nil {
		expr.Count.Accept(v)
		v.space()
		v.write("Mal")
		v.space()
		expr.Value.Accept(v)
	} else {
		v.write("eine leere")
		v.space()
		v.write(expr.Type.String())
	}
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitUnaryExpr(expr *ast.UnaryExpr) ast.VisitResult {
outer:
	switch expr.Operator {
	case ast.UN_ABS:
		v.write("der Betrag von")
		v.space()
	case ast.UN_LEN:
		v.write("die Länge von")
		v.space()
	case ast.UN_NEGATE:
		v.write("-")
	case ast.UN_NOT:
		switch rhs := expr.Rhs.(type) {
		case *ast.FuncCall:
			if rhs.Alias.Negated {
				break outer
			}
		case *ast.TypeCheck:
			v.typeCheck(rhs, true)
			return ast.VisitRecurse
		}
		v.write("nicht")
		v.space()
	case ast.UN_LOGIC_NOT:
		v.write("logisch nicht")
		v.space()
	}
	expr.Rhs.Accept(v)
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBinaryExpr(expr *ast.BinaryExpr) ast.VisitResult {
	switch expr.Operator {
	case ast.BIN_LOG:
		v.write("der Logarithmus von")
		v.space()
		expr.Lhs.Accept(v)
		v.space()
		v.write("zur Basis")
		v.space()
		expr.Rhs.Accept(v)
	case ast.BIN_XOR:
		v.write("entweder")
		v.space()
		expr.Lhs.Accept(v)
		v.write(", oder")
		v.space()
		expr.Rhs.Accept(v)
	case ast.BIN_FIELD_ACCESS:
		expr.Lhs.Accept(v)
		v.space()
		v.write("von")
		v.space()
		expr.Rhs.Accept(v)
	case ast.BIN_SLICE_TO:
		expr.Lhs.Accept(v)
		v.space()
		v.write("bis zum")
		v.space()
		expr.Rhs.Accept(v)
		v.write(". Element")
	case ast.BIN_SLICE_FROM:
		expr.Lhs.Accept(v)
		v.space()
		v.write("ab dem")
		v.space()
		expr.Rhs.Accept(v)
		v.write(". Element")
	case ast.BIN_LEFT_SHIFT:
		expr.Lhs.Accept(v)
		v.space()
		v.write("um")
		v.space()
		expr.Rhs.Accept(v)
		v.space()
		v.write("Bit nach links verschoben")
	case ast.BIN_RIGHT_SHIFT:
		expr.Lhs.Accept(v)
		v.space()
		v.write("um")
		v.space()
		expr.Rhs.Accept(v)
		v.space()
		v.write("Bit nach rechts verschoben")
	case ast.BIN_EQUAL, ast.BIN_UNEQUAL, ast.BIN_LESS, ast.BIN_GREATER, ast.BIN_LESS_EQ, ast.BIN_GREATER_EQ:
		expr.Lhs.Accept(v)
		v.space()
		v.write(expr.Operator.String())
		v.space()
		expr.Rhs.Accept(v)
		v.space()
		v.write("ist")
	case ast.BIN_POW:
		if rhs, ok := expr.Rhs.(*ast.BinaryExpr); ok && rhs.Operator == ast.BIN_DIV {
			if div, isDiv := rhs.Lhs.(*ast.IntLit); isDiv && div.Value == 1 {
				v.write("die")
				v.space()
				rhs.Rhs.Accept(v)
				v.write(".")
				v.space()
				v.write("Wurzel von")
				v.space()
				expr.Lhs.Accept(v)
				break
			}
		}

		fallthrough
	default:
		expr.Lhs.Accept(v)
		v.space()
		v.write(expr.Operator.String())
		v.space()
		expr.Rhs.Accept(v)
	}
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitTernaryExpr(expr *ast.TernaryExpr) ast.VisitResult {
	switch expr.Operator {
	case ast.TER_SLICE:
		expr.Lhs.Accept(v)
		v.space()
		v.write("im Bereich von")
		v.space()
		expr.Mid.Accept(v)
		v.space()
		v.write("bis")
		v.space()
		expr.Rhs.Accept(v)
	case ast.TER_BETWEEN:
		expr.Lhs.Accept(v)
		v.space()
		v.write("zwischen")
		v.space()
		expr.Mid.Accept(v)
		v.space()
		v.write("und")
		v.space()
		expr.Rhs.Accept(v)
		v.space()
		v.write("ist")
	case ast.TER_FALLS:
		expr.Lhs.Accept(v)
		v.write(", falls")
		v.space()
		expr.Mid.Accept(v)
		v.write(", ansonsten")
		v.space()
		expr.Rhs.Accept(v)
	}
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitCastExpr(expr *ast.CastExpr) ast.VisitResult {
	expr.Lhs.Accept(v)
	v.space()
	v.write("als")
	v.space()
	v.write(expr.TargetType.String())
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitCastAssigneable(expr *ast.CastAssigneable) ast.VisitResult {
	expr.Lhs.Accept(v)
	v.space()
	v.write("als")
	v.space()
	v.write(expr.TargetType.String())
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitTypeOpExpr(expr *ast.TypeOpExpr) ast.VisitResult {
	switch expr.Operator {
	case ast.TYPE_SIZE:
		v.write("die Größe von")
		v.space()
	case ast.TYPE_DEFAULT:
		v.write("der Standardwert von")
		v.space()
	}
	v.write(dativIndefiniteArticle(expr.Rhs.Gender()))
	v.space()
	v.write(expr.Rhs.String())
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) typeCheck(expr *ast.TypeCheck, negated bool) {
	expr.Lhs.Accept(v)
	v.space()
	if negated {
		v.write("k")
	}
	v.write(nominativIndefiniteArticle(expr.CheckType.Gender()))
	v.space()
	v.write(expr.CheckType.String())
	v.space()
	v.write("ist")
	v.advanceTo(expr.GetRange().End)
}

func (v *formattingVisitor) VisitTypeCheck(expr *ast.TypeCheck) ast.VisitResult {
	v.typeCheck(expr, false)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitGrouping(expr *ast.Grouping) ast.VisitResult {
	v.write("(")
	expr.Expr.Accept(v)
	v.write(")")
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitFuncCall(expr *ast.FuncCall) ast.VisitResult {
	if expr.Func != nil && len(expr.Func.Aliases) > 0 {
		v.writeAliasCall(expr.Alias.GetTokens(), expr.Args)
	} else {
		v.write(v.getRangeText(expr.GetRange()))
	}

	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitStructLiteral(expr *ast.StructLiteral) ast.VisitResult {
	if expr.Struct != nil && len(expr.Struct.Aliases) > 0 {
		v.writeAliasCall(expr.Alias.Tokens, expr.Args)
	} else {
		v.write(v.getRangeText(expr.GetRange()))
	}
	v.advanceTo(expr.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBadStmt(stmt *ast.BadStmt) ast.VisitResult {
	v.write(stmt.Tok.Literal)
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitDeclStmt(stmt *ast.DeclStmt) ast.VisitResult {
	stmt.Decl.Accept(v)
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitExprStmt(stmt *ast.ExprStmt) ast.VisitResult {
	stmt.Expr.Accept(v)
	if _, isBad := stmt.Expr.(*ast.BadExpr); !isBad {
		v.write(".")
	}
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitImportStmt(stmt *ast.ImportStmt) ast.VisitResult {
	v.write("Binde")
	v.space()
	switch {
	case stmt.IsDirectoryImport:
		if stmt.IsRecursive {
			v.write("rekursiv alle Module aus")
			v.space()
		} else {
			v.write("alle Module aus")
			v.space()
		}
	case len(stmt.ImportedSymbols) > 0:
		for i, sym := range stmt.ImportedSymbols {
			if i > 0 {
				if i == len(stmt.ImportedSymbols)-1 {
					v.space()
					v.write("und")
					v.space()
				} else {
					v.write(", ")
				}
			}
			v.write(sym.Literal)
		}
		v.space()
		v.write("aus")
		v.space()
	}
	v.write(stmt.FileName.Literal)
	v.space()
	v.write("ein.")
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) litAssign(stmt *ast.AssignStmt) {
	stmt.Var.Accept(v)
	v.space()
	v.write("ist")
	v.space()
	stmt.Rhs.Accept(v)
}

func (v *formattingVisitor) negateAssign(stmt *ast.AssignStmt) {
	v.write("Negiere")
	v.space()
	stmt.Var.Accept(v)
}

func (v *formattingVisitor) compoundAssign(stmt *ast.AssignStmt, operator ast.BinaryOperator, other ast.Expression) {
	switch operator {
	case ast.BIN_PLUS, ast.BIN_MINUS, ast.BIN_MULT, ast.BIN_DIV:
		switch operator {
		case ast.BIN_PLUS:
			v.write("Erhöhe")
		case ast.BIN_MINUS:
			v.write("Verringere")
		case ast.BIN_MULT:
			v.write("Vervielfache")
		case ast.BIN_DIV:
			v.write("Teile")
		}
		v.space()
		stmt.Var.Accept(v)
		v.space()
		if operator == ast.BIN_DIV {
			v.write("durch")
		} else {
			v.write("um")
		}
		v.space()
		other.Accept(v)
	case ast.BIN_LEFT_SHIFT, ast.BIN_RIGHT_SHIFT:
		v.write("Verschiebe")
		v.space()
		stmt.Var.Accept(v)
		v.space()
		v.write("um")
		v.space()
		other.Accept(v)
		v.space()
		if operator == ast.BIN_LEFT_SHIFT {
			v.write("Bit nach links")
		} else {
			v.write("Bit nach rechts")
		}
	}
}

func (v *formattingVisitor) normalAssign(stmt *ast.AssignStmt) {
	v.write("Speichere")
	v.space()
	stmt.Rhs.Accept(v)
	v.space()
	v.write("in")
	v.space()
	stmt.Var.Accept(v)
}

func (v *formattingVisitor) assign(stmt *ast.AssignStmt, inWhile bool) {
	switch stmt.Var.(type) {
	case *ast.CastAssigneable, *ast.FieldAccess:
		v.normalAssign(stmt)
	default:
		switch rhs := stmt.Rhs.(type) {
		case ast.Literal:
			if stmt.Tok.Type == token.SPEICHERE {
				v.normalAssign(stmt)
			} else {
				v.litAssign(stmt)
			}

		case *ast.UnaryExpr:
			if rhs.Operator == ast.UN_NOT || rhs.Operator == ast.UN_NEGATE {
				v.negateAssign(stmt)
			}

		case *ast.BinaryExpr:
			switch stmt.Var {
			case rhs.Lhs:
				v.compoundAssign(stmt, rhs.Operator, rhs.Rhs)
			case rhs.Rhs:
				v.compoundAssign(stmt, rhs.Operator, rhs.Lhs)
			default:
				v.normalAssign(stmt)
			}

		default:
			v.normalAssign(stmt)
		}
	}

	if !inWhile {
		v.write(".")
	}
}

func (v *formattingVisitor) VisitAssignStmt(stmt *ast.AssignStmt) ast.VisitResult {
	v.assign(stmt, false)

	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBlockStmt(stmt *ast.BlockStmt) ast.VisitResult {
	v.write(":")
	v.indent++
	for _, s := range stmt.Statements {
		v.newline()
		s.Accept(v)
	}
	v.indent--
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) maybeCompactBlock(stmt *ast.BlockStmt, blockPrefix string, inlinePrefix string) bool {
	if len(stmt.Statements) == 1 && stmt.Colon.Type != token.COLON {
		v.write(inlinePrefix)
		v.space()
		v.nextLetterLower = true
		stmt.Statements[0].Accept(v)
		return true
	} else {
		v.write(blockPrefix)
		stmt.Accept(v)
		return false
	}
}

func (v *formattingVisitor) writeIfStmt(stmt *ast.IfStmt, elseIf bool) {
	if elseIf {
		v.write("Wenn aber")
	} else {
		v.write("Wenn")
	}
	v.space()
	stmt.Condition.Accept(v)
	v.maybeCompactBlock(stmt.Then.(*ast.BlockStmt), ", dann", ",")

	if stmt.Else != nil {
		v.newline()
		if nestedIf, ok := stmt.Else.(*ast.IfStmt); ok {
			v.writeIfStmt(nestedIf, true)
		} else {
			v.write("Sonst")
			v.maybeCompactBlock(stmt.Else.(*ast.BlockStmt), "", "")
		}
	}
}

func (v *formattingVisitor) VisitIfStmt(stmt *ast.IfStmt) ast.VisitResult {
	v.writeIfStmt(stmt, false)
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

// whether expr is the literal 1 (or 1.0), the default for-loop step-size
func isDefaultStep(expr ast.Expression) bool {
	switch e := expr.(type) {
	case *ast.IntLit:
		return e.Value == 1
	case *ast.FloatLit:
		return e.Value == 1.0
	}
	return false
}

func (v *formattingVisitor) VisitWhileStmt(stmt *ast.WhileStmt) ast.VisitResult {
	switch stmt.While.Type {
	case token.MACHE: // Mache: <body> Solange <cond>.
		v.write("Mache")
		stmt.Body.Accept(v)
		v.newline()
		v.write("Solange")
		v.space()
		stmt.Condition.Accept(v)
		v.write(".")
	case token.WIEDERHOLE: // Wiederhole: <body> <count> Mal.
		switch body := stmt.Body.(type) {
		case *ast.BlockStmt:
			v.write("Wiederhole")
			stmt.Body.Accept(v)
			v.newline()
			stmt.Condition.Accept(v)
			v.space()
			v.write("Mal.")
		case *ast.ExprStmt:
			body.Expr.Accept(v)
			v.space()
			stmt.Condition.Accept(v)
			v.space()
			v.write("Mal.")
		case *ast.AssignStmt:
			v.assign(body, true)
			v.space()
			stmt.Condition.Accept(v)
			v.space()
			v.write("Mal.")

		default:
			stmt.Body.Accept(v)
			v.space()
			stmt.Condition.Accept(v)
			v.space()
			v.write("Mal.")
		}

		v.advanceTo(stmt.GetRange().End)
		return ast.VisitRecurse
	default: // Solange <cond>, mache: <body>
		v.write("Solange")
		v.space()
		stmt.Condition.Accept(v)
		v.maybeCompactBlock(stmt.Body.(*ast.BlockStmt), ", mache", ",")
	}
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitForStmt(stmt *ast.ForStmt) ast.VisitResult {
	v.write("Für")
	v.space()
	v.write(forPronoun(stmt.Initializer.Type.Gender()))
	v.space()
	v.write(stmt.Initializer.Type.String())
	v.space()
	v.write(stmt.Initializer.Name())
	v.space()
	v.write("von")
	v.space()
	stmt.Initializer.InitVal.Accept(v)
	v.space()
	v.write("bis")
	v.space()
	stmt.To.Accept(v)
	if stmt.StepSize != nil && !isDefaultStep(stmt.StepSize) {
		v.space()
		v.write("mit Schrittgröße")
		v.space()
		stmt.StepSize.Accept(v)
	}

	v.maybeCompactBlock(stmt.Body, ", mache", ",")
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitForRangeStmt(stmt *ast.ForRangeStmt) ast.VisitResult {
	v.write("Für")
	v.space()
	v.write(forPronoun(stmt.Initializer.Type.Gender()))
	v.space()
	if stmt.Initializer.Type == ddptypes.BUCHSTABE {
		v.write("Buchstaben")
	} else {
		v.write(stmt.Initializer.Type.String())
	}
	v.space()
	v.write(stmt.Initializer.Name())
	if stmt.Index != nil {
		v.space()
		v.write("mit Index")
		v.space()
		v.write(stmt.Index.Name())
	}
	v.space()
	v.write("in")
	v.space()
	stmt.In.Accept(v)
	v.maybeCompactBlock(stmt.Body, ", mache", ",")
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitBreakContinueStmt(stmt *ast.BreakContinueStmt) ast.VisitResult {
	if stmt.Tok.Type == token.VERLASSE {
		v.write("Verlasse die Schleife.")
	} else {
		v.write("Fahre mit der Schleife fort.")
	}
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitReturnStmt(stmt *ast.ReturnStmt) ast.VisitResult {
	if stmt.Value == nil {
		v.write("Verlasse die Funktion.")
	} else {
		v.write("Gib")
		v.space()
		stmt.Value.Accept(v)
		v.space()
		v.write("zurück.")
	}
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

func (v *formattingVisitor) VisitTodoStmt(stmt *ast.TodoStmt) ast.VisitResult {
	v.write("...")
	v.advanceTo(stmt.GetRange().End)
	return ast.VisitRecurse
}

// returns the text of v.document that lies within rang
func (v *formattingVisitor) getRangeText(rang token.Range) string {
	rang = token.Range{
		Start: token.Position{
			Line:   uint(rang.Start.Line - 1),
			Column: uint(rang.Start.Column - 1),
		},
		End: token.Position{
			Line:   uint(rang.End.Line - 1),
			Column: uint(rang.End.Column - 1),
		},
	}

	lines := strings.Split(v.document, "\n")

	if rang.Start.Line == rang.End.Line {
		line := []rune(lines[rang.Start.Line])
		return string(line[rang.Start.Column:rang.End.Column])
	}

	var text strings.Builder
	startLine := []rune(lines[rang.Start.Line])
	text.WriteString(string(startLine[rang.Start.Column:]))
	for i := rang.Start.Line + 1; i < rang.End.Line; i++ {
		text.WriteByte('\n')
		text.WriteString(lines[i])
	}
	text.WriteByte('\n')
	endLine := []rune(lines[rang.End.Line])
	text.WriteString(string(endLine[:rang.End.Column]))

	return text.String()
}
