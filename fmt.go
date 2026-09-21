package formatierer

import (
	"strings"
	"unicode/utf8"

	"github.com/DDP-Projekt/Kompilierer/src/ast"
	"github.com/DDP-Projekt/Kompilierer/src/token"
)

func GetFormattedDocument(document string, module ast.Module, options FormattingOptions) string {
	visitor := &formattingVisitor{
		document:   document,
		currentPos: token.Position{Line: 1, Column: 1},
		opts:       options,
	}
	statements := module.Ast.Statements

	for _, stmt := range statements {
		// TODO: cuts off this comment in tictactoe: "    [Ansonsten] Gib 2 zurück."
		between := visitor.getRangeText(token.Range{
			Start: visitor.currentPos,
			End:   stmt.GetRange().Start,
		})

		visitor.write(between)
		//log.Infof(between)
		//log.Infof(visitor.currentPos.String())

		stmt.Accept(visitor)
	}

	//log.Infof(visitor.currentPos.String())
	lines := strings.Split(document, "\n")

	between := visitor.getRangeText(token.Range{
		Start: visitor.currentPos,
		End: token.Position{
			Line:   uint(len(lines)),
			Column: uint(utf8.RuneCountInString(lines[len(lines)-1])) + 1,
		},
	})
	visitor.write(between)

	return visitor.out.String()
}
