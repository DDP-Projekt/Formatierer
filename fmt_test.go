package formatierer

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DDP-Projekt/Kompilierer/src/ddperror"
	"github.com/DDP-Projekt/Kompilierer/src/parser"
	"github.com/aymanbagabas/go-udiff"
)

var (
	stateMu sync.Mutex
	// the formatted output of every test's source, set by setTest
	results = map[*testing.T]string{}
	// the directory the test's document lives in, see testDir
	dirs = map[*testing.T]string{}
)

// returns the (temporary) directory in which the test's document is located,
// so that tests can place modules next to it with writeModule
func testDir(t *testing.T) string {
	t.Helper()
	stateMu.Lock()
	defer stateMu.Unlock()
	dir, ok := dirs[t]
	if !ok {
		dir = t.TempDir()
		dirs[t] = dir
		t.Cleanup(func() {
			stateMu.Lock()
			defer stateMu.Unlock()
			delete(dirs, t)
		})
	}
	return dir
}

// writes a module with the given content to relPath (relative to the test's
// document), so that the test's source can import it
func writeModule(t *testing.T, relPath, content string) {
	t.Helper()
	path := filepath.Join(testDir(t), relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// parses and formats source, so that checkResult can compare against the output.
// The source must be valid DDP, otherwise the test fails.
func setTestImpl(t *testing.T, source string, checkErrs bool) {
	t.Helper()

	path := filepath.Join(testDir(t), "test.ddp")

	var errs []ddperror.Error
	module, err := parser.Parse(parser.Options{
		FileName: path,
		Source:   []byte(source),
		ErrorHandler: func(e ddperror.Error) {
			errs = append(errs, e)
		},
	})
	if err != nil {
		t.Fatalf("parsing test source: %v", err)
	}
	if checkErrs {
		for _, e := range errs {
			if e.Level == ddperror.LEVEL_ERROR {
				t.Errorf("test source is invalid at %s: %s", e.Range.Start, e.Msg)
			}
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	formatted, err := GetFormattedDocument(source, module, FormattingOptions{
		InsertSpaces: false,
	})
	if err != nil {
		t.Errorf("error formatting: %s", err)
	}

	stateMu.Lock()
	defer stateMu.Unlock()
	results[t] = formatted
	t.Cleanup(func() {
		stateMu.Lock()
		defer stateMu.Unlock()
		delete(results, t)
	})
}

func setTest(t *testing.T, source string) {
	setTestImpl(t, source, true)
}

func setTestUnchecked(t *testing.T, source string) {
	setTestImpl(t, source, false)
}

// asserts that the formatter produced exactly expected for the source given to setTest
func checkResult(t *testing.T, expected string) {
	t.Helper()

	stateMu.Lock()
	got, ok := results[t]
	stateMu.Unlock()
	if !ok {
		t.Fatal("checkResult called before setTest")
	}

	if got != expected {
		t.Errorf("formatted output differs from expected:\n%s", udiff.Unified("expected", "got", expected, got))
	}
}

/*
	Bad
*/

func TestBadExpr(t *testing.T) {
	src := `2 um 5 Bit nach x`
	setTestUnchecked(t, src)
	checkResult(t, src)
}

func TestBadStmt(t *testing.T) {
	src := `Binde`
	setTestUnchecked(t, src)
	checkResult(t, src)
}

func TestBadDecl(t *testing.T) {
	src := `Die aowidh`
	setTestUnchecked(t, src)
	checkResult(t, src)
}

/*
	Kommentare
*/

func TestModuleComment(t *testing.T) {
	src := `[
	Modul-Kommentar am Dateianfang.
]

Die Zahl z ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestSingleLineComment(t *testing.T) {
	src := `[Einzeiliger Kommentar]
Die Zahl z ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNestedSingleLineComment(t *testing.T) {
	src := `[[Verschachtelter einzeiliger Kommentar]]
Die Zahl z ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestMultiLineComment(t *testing.T) {
	src := `[
	Mehrzeiliger Kommentar
	[
		mit verschachteltem Abschnitt
	]
]
Die Zahl z ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCommentBetweenStatements(t *testing.T) {
	src := `Die Zahl a ist 1.

[Kommentar zwischen zwei Anweisungen]
Die Zahl b ist 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTrailingComment(t *testing.T) {
	src := `Die Zahl a ist 1. [Kommentar hinter einer Anweisung]
Die Zahl b ist 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCommentAtEnd(t *testing.T) {
	src := `Die Zahl a ist 1.

[Kommentar am Dateiende]`
	setTest(t, src)
	checkResult(t, src)
}

func TestCommentOnly(t *testing.T) {
	src := `[Eine Datei, die nur aus einem Kommentar besteht]`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Einbindungen
*/

// a module with public constants and functions, in the spirit of "Duden/Mathe"
const matheModule = `Die öffentliche Konstante PI ist 3,14.
Die öffentliche Konstante E ist 2,71.

Die öffentliche Funktion Max mit den Parametern a und b vom Typ Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib a, falls a größer als b ist, ansonsten b zurück.
Und kann so benutzt werden:
	"das Maximum von <a> und <b>"

Die öffentliche Funktion Min mit den Parametern a und b vom Typ Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib a, falls a kleiner als b ist, ansonsten b zurück.
Und kann so benutzt werden:
	"das Minimum von <a> und <b>"

Die öffentliche Funktion Max3 mit den Parametern a, b und c vom Typ Zahl, Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib das Maximum von (das Maximum von a und b) und c zurück.
Und kann so benutzt werden:
	"das Maximum von <a>, <b> und <c>"
`

// a module with a single public constant, in the spirit of "Duden/Zeichen"
const zeichenModule = `Die öffentliche Konstante Leerzeichen ist ' '.
`

// a module whose functions share an alias for different parameter types,
// in the spirit of "Duden/Ausgabe"
const ausgabeModule = `Die öffentliche Funktion Schreibe_Zahl mit dem Parameter p1 vom Typ Zahl, gibt nichts zurück, macht:
	Die Zahl n ist p1.
Und kann so benutzt werden:
	"Schreibe <p1> auf eine Zeile"

Die öffentliche Funktion Schreibe_Text mit dem Parameter p1 vom Typ Text, gibt nichts zurück, macht:
	Die Zahl n ist die Länge von p1.
Und kann so benutzt werden:
	"Schreibe <p1> auf eine Zeile"
`

func TestImportModule(t *testing.T) {
	writeModule(t, "Ausgabe.ddp", ausgabeModule)
	src := `Binde "Ausgabe" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportModuleFromSubdirectory(t *testing.T) {
	writeModule(t, "Module/Ausgabe.ddp", ausgabeModule)
	src := `Binde "Module/Ausgabe" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportTwoSymbols(t *testing.T) {
	writeModule(t, "Mathe.ddp", matheModule)
	src := `Binde PI und E aus "Mathe" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportSingleSymbol(t *testing.T) {
	writeModule(t, "Zeichen.ddp", zeichenModule)
	src := `Binde Leerzeichen aus "Zeichen" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportSymbolList(t *testing.T) {
	writeModule(t, "Mathe.ddp", matheModule)
	src := `Binde Max, Min und Max3 aus "Mathe" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportDirectory(t *testing.T) {
	writeModule(t, "Module/A.ddp", "Die öffentliche Zahl a ist 1.\n")
	writeModule(t, "Module/B.ddp", "Die öffentliche Zahl b ist 2.\n")
	src := `Binde alle Module aus "Module" ein.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportDirectoryRecursive(t *testing.T) {
	writeModule(t, "Module/A.ddp", "Die öffentliche Zahl a ist 1.\n")
	writeModule(t, "Module/Unter/B.ddp", "Die öffentliche Zahl b ist 2.\n")
	src := `Binde rekursiv alle Module aus "Module" ein.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Konstanten
*/

func TestConstDecl(t *testing.T) {
	src := `Die Konstante MAX ist 100.`
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicConstDecl(t *testing.T) {
	src := `Die öffentliche Konstante GRUSS ist "Hallo".`
	setTest(t, src)
	checkResult(t, src)
}

func TestConstDeclListLit(t *testing.T) {
	src := `Die Konstante ZIFFERN ist eine Liste, die aus 1, 2, 3 besteht.`
	setTest(t, src)
	checkResult(t, src)
}

func TestConstDeclRepeatedListLit(t *testing.T) {
	src := `Die Konstante NULLEN ist 3 Mal 0.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Variablen
*/

func TestVarDeclZahl(t *testing.T) {
	src := `Die Zahl z ist 42.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclKommazahl(t *testing.T) {
	src := `Die Kommazahl k ist 3,5.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclByte(t *testing.T) {
	src := `Der Byte b ist 7.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclWahrheitswert(t *testing.T) {
	src := `Der Wahrheitswert w ist wahr.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclBuchstabe(t *testing.T) {
	src := `Der Buchstabe c ist 'A'.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclText(t *testing.T) {
	src := `Der Text t ist "Welt".`
	setTest(t, src)
	checkResult(t, src)
}

func TestEscapeSequences(t *testing.T) {
	src := `Der Text mit_escapes ist "Zeile\nTabulator\tAnfuehrungszeichen\" Backslash\\".
Der Buchstabe zeilenumbruch ist '\n'.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclVariable(t *testing.T) {
	src := `Die Variable v ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestVarDeclList(t *testing.T) {
	src := `Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3, 4 besteht.`
	setTest(t, src)
	checkResult(t, src)
}

func TestEmptyListLit(t *testing.T) {
	src := `Die Text Liste tl ist eine leere Text Liste.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatedListLit(t *testing.T) {
	src := `Die Buchstaben Liste bl ist 4 Mal 'x'.`
	setTest(t, src)
	checkResult(t, src)
}

func TestOtherListTypes(t *testing.T) {
	src := `Die Byte Liste byl ist eine leere Byte Liste.
Die Variablen Liste vl ist eine leere Variablen Liste.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBoolDeclTrueIf(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert gross ist wahr, wenn z größer als 10 ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBoolDeclFalseIf(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert klein ist falsch, wenn z größer als 10 ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBoolDeclTernaryTrueIf(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert gross ist wahr, wenn z zwischen 2 und 10 ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBoolDeclTernaryFalseIf(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert gross ist falsch, wenn z zwischen 2 und 10 ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicVarDecl(t *testing.T) {
	src := `Die öffentliche Zahl global_z ist 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestExternVisibleVarDecl(t *testing.T) {
	src := `Die extern sichtbare Zahl extern_z ist 2.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Typen
*/

func TestTypeAliasDecl(t *testing.T) {
	src := `Wir nennen eine Zahl auch eine Hausnummer.`
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicTypeAliasDecl(t *testing.T) {
	src := `Wir nennen einen Text öffentlich auch einen Namen.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTypeDefDecl(t *testing.T) {
	src := `Wir definieren eine Kennzahl als eine Zahl.`
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicTypeDefDecl(t *testing.T) {
	src := `Wir definieren einen Zeiger öffentlich als eine Zahl.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Kombinationen
*/

const vektor2Decl = `Wir nennen die Kombination aus
	der Zahl x mit Standardwert 0,
	der öffentlichen Zahl y mit Standardwert 0,
einen Vektor2, und erstellen sie so:
	"Nullvektor2",
	"ein Vektor2 mit x gleich <x>" oder
	"ein Vektor2 mit x gleich <x> und y gleich <y>"`

const vektor3Decl = `Wir nennen die öffentliche Kombination aus
	dem Vektor2 basis mit Standardwert Nullvektor2,
	der Zahl hoehe mit Standardwert 0,
einen Vektor3, und erstellen sie so:
	"Nullvektor3" oder
	"ein Vektor3 mit basis gleich <basis> und hoehe gleich <hoehe>"`

const buchDecl = `Wir nennen die Kombination aus
	dem Text titel,
	der Zahl seiten,
ein Buch, und erstellen sie so:
	"ein Buch mit titel gleich <titel> und seiten gleich <seiten>" oder
	"ein Buch namens <titel> mit <seiten> Seiten"`

const zweierDecl = `Wir nennen die generische Kombination aus
	dem T erster,
	dem T zweiter,
einen Zweier, und erstellen sie so:
	"ein Zweier aus <erster> und <zweiter>"`

func TestStructDecl(t *testing.T) {
	src := vektor2Decl
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicStructDeclWithStructField(t *testing.T) {
	src := vektor2Decl + "\n\n" + vektor3Decl
	setTest(t, src)
	checkResult(t, src)
}

func TestStructDeclWithoutDefaults(t *testing.T) {
	src := buchDecl
	setTest(t, src)
	checkResult(t, src)
}

func TestGenericStructDecl(t *testing.T) {
	src := zweierDecl
	setTest(t, src)
	checkResult(t, src)
}

func TestStructVarDecl(t *testing.T) {
	src := vektor2Decl + `

Der Vektor2 vec2 ist Nullvektor2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestStructLiteralAllFields(t *testing.T) {
	src := vektor2Decl + "\n\n" + vektor3Decl + `

Der Vektor3 vec3 ist ein Vektor3 mit basis gleich (Nullvektor2) und hoehe gleich 9.`
	setTest(t, src)
	checkResult(t, src)
}

func TestGenericStructInstantiation(t *testing.T) {
	src := zweierDecl + `

Der Zahl-Zweier paar ist ein Zweier aus 1 und 2.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Funktionen
*/

func TestFuncDeclNoParams(t *testing.T) {
	src := `Die Funktion Begruessung gibt einen Text zurück, macht:
	Gib "Hallo Welt" zurück.
Und kann so benutzt werden:
	"Begrüßung"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclOneParam(t *testing.T) {
	src := `Die Funktion Verdoppeln mit dem Parameter n vom Typ Zahl, gibt eine Zahl zurück, macht:
	Gib n mal 2 zurück.
Und kann so benutzt werden:
	"<n> verdoppelt"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclTwoParams(t *testing.T) {
	src := `Die Funktion Addieren mit den Parametern a und b vom Typ Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib a plus b zurück.
Und kann so benutzt werden:
	"<a> addiert mit <b>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclTwoAliases(t *testing.T) {
	src := `Die Funktion fn gibt nichts zurück, macht:
Und kann so benutzt werden:
	"abc" oder
	"def"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclMultipleAliases(t *testing.T) {
	src := `Die Funktion fn gibt nichts zurück, macht:
Und kann so benutzt werden:
	"abc",
	"def" oder
	"xyz"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclMultipleAliasesCorrectFormat(t *testing.T) {
	src := `Die Funktion fn gibt nichts zurück, macht:
Und kann so benutzt werden:
	"abc","def","xyz"`

	res := `Die Funktion fn gibt nichts zurück, macht:
Und kann so benutzt werden:
	"abc",
	"def" oder
	"xyz"`
	setTest(t, src)
	checkResult(t, res)
}

func TestFuncDeclThreeParams(t *testing.T) {
	src := `Die Funktion Mitte mit den Parametern a, b und c vom Typ Zahl, Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib ((a plus b plus c) durch 3) als Zahl zurück.
Und kann so benutzt werden:
	"die Mitte von <a>, <b> und <c>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclReferenceParam(t *testing.T) {
	src := `Die Funktion Erhoehen mit dem Parameter r vom Typ Zahlen Referenz, gibt nichts zurück, macht:
	Speichere r plus 1 in r.
Und kann so benutzt werden:
	"inkrementiere <r>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclListParamAndReturn(t *testing.T) {
	src := `Die Funktion Anhaengen mit den Parametern l und e vom Typ Zahlen Liste und Zahl, gibt eine Zahlen Liste zurück, macht:
	Gib l verkettet mit e zurück.
Und kann so benutzt werden:
	"<l> mit <e> dahinter"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclListReferenceParam(t *testing.T) {
	src := `Die Funktion Liste_Leeren mit dem Parameter l vom Typ Zahlen Listen Referenz, gibt nichts zurück, macht:
	Speichere eine leere Zahlen Liste in l.
Und kann so benutzt werden:
	"leere <l>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclVoidEarlyReturn(t *testing.T) {
	src := `Die Funktion Melden mit dem Parameter m vom Typ Text, gibt nichts zurück, macht:
	Wenn m gleich "" ist, dann:
		Verlasse die Funktion.
	Die Zahl n ist die Länge von m.
Und kann so benutzt werden:
	"melde <m>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestPublicExternVisibleFuncDecl(t *testing.T) {
	src := `Die öffentliche Funktion Quadrat mit dem Parameter n vom Typ Zahl, gibt eine Zahl zurück, ist extern sichtbar, macht:
	Gib n mal n zurück.
Und kann so benutzt werden:
	"das Quadrat von <n>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclNegatedAlias(t *testing.T) {
	src := `Die Funktion Ist_Gerade mit dem Parameter n vom Typ Zahl, gibt einen Wahrheitswert zurück, macht:
	Gib n modulo 2 gleich 0 ist zurück.
Und kann so benutzt werden:
	"<n> ist <!nicht> gerade"`
	setTest(t, src)
	checkResult(t, src)
}

func TestExternFuncDecl(t *testing.T) {
	src := `Die Funktion Externe_Zahl gibt eine Zahl zurück,
ist in "extern.c" definiert
Und kann so benutzt werden:
	"die externe Zahl"`
	setTest(t, src)
	checkResult(t, src)
}

// a forward declaration is only valid together with its later definition
func TestForwardFuncDeclAndFuncDef(t *testing.T) {
	src := `Die Funktion Nachtrag mit dem Parameter n vom Typ Zahl, gibt eine Zahl zurück,
wird später definiert
Und kann so benutzt werden:
	"<n> nachgetragen"

Die Funktion Nachtrag macht:
	Gib n plus 1 zurück.`
	setTest(t, src)
	checkResult(t, src)
}

func TestGenericFuncDecl(t *testing.T) {
	src := `Die generische Funktion Tausche mit den Parametern a und b vom Typ T Referenz und T Referenz, gibt nichts zurück, macht:
	Das T temp ist a.
	Speichere b in a.
	Speichere temp in b.
Und kann so benutzt werden:
	"tausche <a> und <b>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestGenericFuncDeclGenericReturn(t *testing.T) {
	src := `Die generische Funktion Erstes mit dem Parameter l vom Typ T Liste, gibt ein T zurück, macht:
	Gib l an der Stelle 1 zurück.
Und kann so benutzt werden:
	"das erste Element von <l>"`
	setTest(t, src)
	checkResult(t, src)
}

func TestUnaryOperatorOverload(t *testing.T) {
	src := buchDecl + `

Die Funktion Buch_Laenge mit dem Parameter p vom Typ Buch, gibt eine Zahl zurück, macht:
	Gib seiten von p zurück.
Und überlädt den "Länge" Operator.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBinaryOperatorOverload(t *testing.T) {
	src := buchDecl + `

Die Funktion Buch_Plus mit den Parametern p und n vom Typ Buch und Zahl, gibt eine Zahl zurück, macht:
	Gib seiten von p plus n zurück.
Und überlädt den "plus" Operator.`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncDeclTodoBody(t *testing.T) {
	src := `Die Funktion Unfertig gibt eine Zahl zurück, macht:
	...
Und kann so benutzt werden:
	"etwas Unfertiges"`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Zuweisungen
*/

func TestLiteralAssign(t *testing.T) {
	src := `Die Zahl z ist 42.
z ist 50.`
	setTest(t, src)
	checkResult(t, src)
}

func TestSpeichereAssign(t *testing.T) {
	src := `Die Zahl z ist 42.
Speichere z plus 8 in z.
Speichere (z plus 8) in z.`
	setTest(t, src)
	checkResult(t, src)
}

func TestIndexAssign(t *testing.T) {
	src := `Die Text Liste tl ist eine Liste, die aus "a", "b" besteht.
tl an der Stelle 1 ist "A".
tl an der Stelle 1 ist "A".`
	setTest(t, src)
	checkResult(t, src)
}

func TestSpeichereIndexAssign(t *testing.T) {
	src := `Die Zahlen Liste l ist eine leere Zahlen Liste.
Speichere 5 in (l an der Stelle 1).
Speichere 5 in (l an der Stelle 1).`
	setTest(t, src)
	checkResult(t, src)
}

func TestCastAssign(t *testing.T) {
	src := `Wir definieren eine Kennzahl als eine Zahl.
Die Kennzahl kz ist 1 als Kennzahl.
Speichere 2 in kz als Zahl.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCastReferenceArgument(t *testing.T) {
	src := `Die Funktion Erhoehen mit dem Parameter r vom Typ Zahlen Referenz, gibt nichts zurück, macht:
	Speichere r plus 1 in r.
Und kann so benutzt werden:
	"inkrementiere <r>"

Wir definieren eine Kennzahl als eine Zahl.
Die Kennzahl kz ist 1 als Kennzahl.
inkrementiere (kz als Zahl).`
	setTest(t, src)
	checkResult(t, src)
}

func TestFieldAssign(t *testing.T) {
	src := vektor2Decl + `

Der Vektor2 vec2 ist Nullvektor2.
Speichere 3 in x von vec2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNestedFieldAssign(t *testing.T) {
	src := vektor2Decl + "\n\n" + vektor3Decl + `

Der Vektor3 vec3 ist Nullvektor3.
Speichere 4 in x von basis von vec3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignPlus(t *testing.T) {
	src := `Die Zahl z ist 42.
Erhöhe z um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignMinus(t *testing.T) {
	src := `Die Zahl z ist 42.
Verringere z um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignMult(t *testing.T) {
	src := `Die Zahl z ist 42.
Vervielfache z um 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignDiv(t *testing.T) {
	src := `Die Kommazahl k ist 3,5.
Teile k durch 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignShiftLeft(t *testing.T) {
	src := `Die Zahl z ist 42.
Verschiebe z um 1 Bit nach links.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCompoundAssignShiftRight(t *testing.T) {
	src := `Die Zahl z ist 42.
Verschiebe z um 1 Bit nach rechts.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNegateAssign(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert w ist wahr.
Negiere z.
Negiere w.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Ausdrücke
*/

func TestArithmetic(t *testing.T) {
	src := `Die Zahl a ist 2 plus 3.
Die Zahl b ist 7 minus 3.
Die Zahl c ist 4 mal 5.
Die Kommazahl d ist 9 durch 3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestModulo(t *testing.T) {
	src := `Die Zahl a ist 7 modulo 3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestPower(t *testing.T) {
	src := `Die Kommazahl a ist 2 hoch 8.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRoot(t *testing.T) {
	src := `Die Kommazahl a ist die 2. Wurzel von 16.`
	setTest(t, src)
	checkResult(t, src)
}

func TestLogarithm(t *testing.T) {
	src := `Die Kommazahl a ist der Logarithmus von 100 zur Basis 10.`
	setTest(t, src)
	checkResult(t, src)
}

func TestAbs(t *testing.T) {
	src := `Die Zahl a ist der Betrag von -3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNegate(t *testing.T) {
	src := `Die Zahl z ist 42.
Die Zahl a ist -z.`
	setTest(t, src)
	checkResult(t, src)
}

func TestLength(t *testing.T) {
	src := `Die Zahl a ist die Länge von "Hallo".`
	setTest(t, src)
	checkResult(t, src)
}

func TestGrouping(t *testing.T) {
	src := `Die Zahl a ist (2 plus 3) mal 4.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNestedGrouping(t *testing.T) {
	src := `Die Zahl a ist ((2 plus 3) mal 4) minus 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBitwiseOperators(t *testing.T) {
	src := `Die Zahl a ist logisch nicht 1.
Die Zahl b ist 5 logisch und 3.
Die Zahl c ist 5 logisch oder 3.
Die Zahl d ist 5 logisch kontra 3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBitShiftExpr(t *testing.T) {
	src := `Die Zahl a ist 1 um 4 Bit nach links verschoben.
Die Zahl b ist 16 um 2 Bit nach rechts verschoben.`
	setTest(t, src)
	checkResult(t, src)
}

func TestGreaterLess(t *testing.T) {
	src := `(3 größer als 2 ist).
(2 kleiner als 3 ist).`
	setTest(t, src)
	checkResult(t, src)
}

func TestGreaterLessEqual(t *testing.T) {
	src := `(3 größer als, oder 3 ist).
(2 kleiner als, oder 3 ist).`
	setTest(t, src)
	checkResult(t, src)
}

func TestBetween(t *testing.T) {
	src := `(5 zwischen 1 und 10 ist).`
	setTest(t, src)
	checkResult(t, src)
}

func TestEqualUnequal(t *testing.T) {
	src := `(2 gleich 2 ist).
(2 ungleich 3 ist).`
	setTest(t, src)
	checkResult(t, src)
}

func TestLogicalOperators(t *testing.T) {
	src := `(wahr und falsch).
(wahr oder falsch).
(nicht wahr).`
	setTest(t, src)
	checkResult(t, src)
}

func TestXor(t *testing.T) {
	src := `Der Wahrheitswert a ist entweder wahr, oder falsch.`
	setTest(t, src)
	checkResult(t, src)
}

func TestConcat(t *testing.T) {
	src := `Der Text a ist "Hallo " verkettet mit "Welt".
Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3 besteht.
Die Zahlen Liste b ist zl verkettet mit 5.`
	setTest(t, src)
	checkResult(t, src)
}

func TestIndexing(t *testing.T) {
	src := `Der Buchstabe a ist "Hallo" an der Stelle 1.
Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3 besteht.
Die Zahl b ist zl an der Stelle 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestSlice(t *testing.T) {
	src := `Der Text a ist "Hallo Welt" im Bereich von 1 bis 5.`
	setTest(t, src)
	checkResult(t, src)
}

func TestSliceFrom(t *testing.T) {
	src := `Der Text a ist "Hallo Welt" ab dem 7. Element.`
	setTest(t, src)
	checkResult(t, src)
}

func TestSliceTo(t *testing.T) {
	src := `Der Text a ist "Hallo Welt" bis zum 5. Element.`
	setTest(t, src)
	checkResult(t, src)
}

func TestFieldAccess(t *testing.T) {
	src := vektor2Decl + `

Der Vektor2 vec2 ist Nullvektor2.
Die Zahl a ist x von vec2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNestedFieldAccess(t *testing.T) {
	src := vektor2Decl + "\n\n" + vektor3Decl + `

Der Vektor3 vec3 ist Nullvektor3.
Die Zahl a ist y von basis von vec3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestCast(t *testing.T) {
	src := `Die Zahl z ist 42.
Die Zahl a ist 3,7 als Zahl.
Der Buchstabe b ist 65 als Buchstabe.
Der Text c ist z als Text.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTypeCheck(t *testing.T) {
	src := `Die Variable v ist 1.
Der Wahrheitswert a ist v eine Zahl ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNegatedTypeCheck(t *testing.T) {
	src := `Die Variable v ist 1.
Der Wahrheitswert a ist v kein Text ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTypeSize(t *testing.T) {
	src := `Die Zahl a ist die Größe von einer Zahl.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTypeDefault(t *testing.T) {
	src := `Der Text a ist der Standardwert von einem Text.`
	setTest(t, src)
	checkResult(t, src)
}

func TestTernary(t *testing.T) {
	src := `Der Wahrheitswert w ist wahr.
Die Zahl a ist 1, falls w, ansonsten 2.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Funktionsaufrufe
*/

func TestFuncCallExpr(t *testing.T) {
	src := `Die Funktion Begruessung gibt einen Text zurück, macht:
	Gib "Hallo Welt" zurück.
Und kann so benutzt werden:
	"Begrüßung"

Die Funktion Verdoppeln mit dem Parameter n vom Typ Zahl, gibt eine Zahl zurück, macht:
	Gib n mal 2 zurück.
Und kann so benutzt werden:
	"<n> verdoppelt"

Die Funktion Addieren mit den Parametern a und b vom Typ Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib a plus b zurück.
Und kann so benutzt werden:
	"<a> addiert mit <b>" oder
	"die Summe von <a> und <b>"

Die Funktion Mitte mit den Parametern a, b und c vom Typ Zahl, Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib ((a plus b plus c) durch 3) als Zahl zurück.
Und kann so benutzt werden:
	"die Mitte von <a>, <b> und <c>"

Der Text t ist Begrüßung.
Die Zahl a ist 4 verdoppelt.
Die Zahl b ist 2 addiert mit 3.
Die Zahl c ist die Summe von 2 und 3.
Die Zahl d ist die Mitte von 1, 2 und 3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncCallWithExprArguments(t *testing.T) {
	src := `Die Funktion Addieren mit den Parametern a und b vom Typ Zahl und Zahl, gibt eine Zahl zurück, macht:
	Gib a plus b zurück.
Und kann so benutzt werden:
	"<a> addiert mit <b>"

Die Zahl z ist 1.
Die Zahl a ist (z plus 1) addiert mit (z mal 2).
Die Zahl b ist (2 addiert mit 3) addiert mit 4.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNegatedAliasCall(t *testing.T) {
	src := `Die Funktion Ist_Gerade mit dem Parameter n vom Typ Zahl, gibt einen Wahrheitswert zurück, macht:
	Gib n modulo 2 gleich 0 ist zurück.
Und kann so benutzt werden:
	"<n> ist <!nicht> gerade"

Der Wahrheitswert a ist 4 ist gerade.
Der Wahrheitswert b ist 4 ist nicht gerade.`
	setTest(t, src)
	checkResult(t, src)
}

func TestGenericFuncCall(t *testing.T) {
	src := `Die generische Funktion Erstes mit dem Parameter l vom Typ T Liste, gibt ein T zurück, macht:
	Gib l an der Stelle 1 zurück.
Und kann so benutzt werden:
	"das erste Element von <l>"

Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3 besteht.
Die Zahl a ist das erste Element von zl.`
	setTest(t, src)
	checkResult(t, src)
}

func TestOverloadedOperatorCall(t *testing.T) {
	src := buchDecl + `

Die Funktion Buch_Laenge mit dem Parameter p vom Typ Buch, gibt eine Zahl zurück, macht:
	Gib seiten von p zurück.
Und überlädt den "Länge" Operator.

Die Funktion Buch_Plus mit den Parametern p und n vom Typ Buch und Zahl, gibt eine Zahl zurück, macht:
	Gib seiten von p plus n zurück.
Und überlädt den "plus" Operator.

Das Buch buch ist ein Buch mit titel gleich "DDP" und seiten gleich 120.
Die Zahl a ist die Länge von buch.
Die Zahl b ist buch plus 5.`
	setTest(t, src)
	checkResult(t, src)
}

func TestFuncCallStmt(t *testing.T) {
	src := `Die Funktion Melden mit dem Parameter m vom Typ Text, gibt nichts zurück, macht:
	Die Zahl n ist die Länge von m.
Und kann so benutzt werden:
	"melde <m>"

Die Funktion Erhoehen mit dem Parameter r vom Typ Zahlen Referenz, gibt nichts zurück, macht:
	Speichere r plus 1 in r.
Und kann so benutzt werden:
	"inkrementiere <r>"

Die generische Funktion Tausche mit den Parametern a und b vom Typ T Referenz und T Referenz, gibt nichts zurück, macht:
	Das T temp ist a.
	Speichere b in a.
	Speichere temp in b.
Und kann so benutzt werden:
	"tausche <a> und <b>"

Die Zahl z ist 1.
Die Zahl y ist 2.
melde "Eine Meldung".
inkrementiere z.
tausche z und y.`
	setTest(t, src)
	checkResult(t, src)
}

func TestExprStmt(t *testing.T) {
	src := `Die Funktion Verdoppeln mit dem Parameter n vom Typ Zahl, gibt eine Zahl zurück, macht:
	Gib n mal 2 zurück.
Und kann so benutzt werden:
	"<n> verdoppelt"

21 verdoppelt.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportedFuncCall(t *testing.T) {
	writeModule(t, "Ausgabe.ddp", ausgabeModule)
	src := `Binde "Ausgabe" ein.

Die Zahl z ist 42.
Schreibe "Hallo" auf eine Zeile.
Schreibe z auf eine Zeile.
Schreibe (2 plus 3) auf eine Zeile.`
	setTest(t, src)
	checkResult(t, src)
}

func TestImportedSymbolCall(t *testing.T) {
	writeModule(t, "Mathe.ddp", matheModule)
	src := `Binde Max und PI aus "Mathe" ein.

Die Zahl a ist das Maximum von 1 und 2.
Die Kommazahl b ist PI.`
	setTest(t, src)
	checkResult(t, src)
}

/*
	Kontrollfluss
*/

func TestIfBlock(t *testing.T) {
	src := `Die Zahl z ist 42.
Wenn z größer als 10 ist, dann:
	Erhöhe z um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestIfElseBlock(t *testing.T) {
	src := `Die Zahl z ist 42.
Wenn z größer als 1000 ist, dann:
	Erhöhe z um 1.
Sonst:
	Verringere z um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestElseIfChain(t *testing.T) {
	src := `Die Zahl z ist 42.
Wenn z kleiner als 0 ist, dann:
	Erhöhe z um 1.
Wenn aber z gleich 0 ist, dann:
	Erhöhe z um 2.
Sonst:
	Erhöhe z um 3.`
	setTest(t, src)
	checkResult(t, src)
}

func TestIfSingleStmt(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert w ist wahr.
Wenn w, erhöhe z um 1.
Sonst verringere z um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestIfSingleStmtFollowedByStatement(t *testing.T) {
	src := `Die Zahl z ist 42.
Der Wahrheitswert w ist wahr.
Wenn w, erhöhe z um 1.
Erhöhe z um 2.`
	setTest(t, src)
	checkResult(t, src)
}

func TestWhileBlock(t *testing.T) {
	src := `Die Zahl i ist 3.
Solange i größer als 0 ist, mache:
	Verringere i um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestWhileSingleStmt(t *testing.T) {
	src := `Die Zahl i ist 3.
Solange i größer als 0 ist, verringere i um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestDoWhile(t *testing.T) {
	src := `Die Zahl i ist 0.
Mache:
	Erhöhe i um 1.
Solange i kleiner als 3 ist.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatBlock(t *testing.T) {
	src := `Die Zahl i ist 0.
Wiederhole:
	Erhöhe i um 1.
2 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatSingleCallStmt(t *testing.T) {
	src := `Die Funktion Erhoehen mit dem Parameter r vom Typ Zahlen Referenz, gibt nichts zurück, macht:
	Speichere r plus 1 in r.
Und kann so benutzt werden:
	"inkrementiere <r>"

Die Zahl i ist 0.
inkrementiere i 3 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatSingleExprStmt(t *testing.T) {
	src := `1 3 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatSingleAssignStmt(t *testing.T) {
	src := `Die Zahl x ist 1.
Speichere x plus 1 in x 4 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatSingleAssignLitStmt(t *testing.T) {
	src := `Die Zahl x ist 1.
x ist 5 4 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestRepeatSingleCompoundAssignStmt(t *testing.T) {
	src := `Die Zahl x ist 1.
Erhöhe x um 5 4 Mal.
Verringere x um 5 4 Mal.
Vervielfache x um 5 4 Mal.
Teile x durch 5 4 Mal.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForBlock(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jede Zahl n von 1 bis 3, mache:
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForStepSize(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jede Zahl n von 10 bis 0 mit Schrittgröße -2, mache:
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForSingleStmt(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jede Zahl n von 1 bis 3, erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForFloat(t *testing.T) {
	src := `Die Kommazahl s ist 0,0.
Für jede Kommazahl f von 0,0 bis 1,0 mit Schrittgröße 0,5, mache:
	Erhöhe s um f.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForByte(t *testing.T) {
	src := `Der Byte s ist 0.
Für jeden Byte by von 0 bis 2, mache:
	Erhöhe s um by.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForRangeText(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jeden Buchstaben ch in "abc", mache:
	Erhöhe s um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForRangeList(t *testing.T) {
	src := `Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3 besteht.
Die Zahl s ist 0.
Für jede Zahl n in zl, mache:
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForRangeStructList(t *testing.T) {
	src := buchDecl + `

Das Buch buch ist ein Buch mit titel gleich "DDP" und seiten gleich 120.
Die Buch Liste buecher ist eine Liste, die aus buch besteht.
Die Zahl s ist 0.
Für jedes Buch bu in buecher, mache:
	Erhöhe s um seiten von bu.`
	setTest(t, src)
	checkResult(t, src)
}

func TestForRangeIndex(t *testing.T) {
	src := `Die Zahlen Liste zl ist eine Liste, die aus 1, 2, 3 besteht.
Die Zahl s ist 0.
Für jede Zahl n mit Index idx in zl, mache:
	Erhöhe s um idx.
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestBreak(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jede Zahl n von 1 bis 10, mache:
	Wenn n gleich 3 ist, verlasse die Schleife.
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestContinue(t *testing.T) {
	src := `Die Zahl s ist 0.
Für jede Zahl n von 1 bis 5, mache:
	Wenn n gleich 3 ist, fahre mit der Schleife fort.
	Erhöhe s um n.`
	setTest(t, src)
	checkResult(t, src)
}

func TestStandaloneBlock(t *testing.T) {
	src := `:
	Die Zahl lokal ist 1.
	Erhöhe lokal um 1.`
	setTest(t, src)
	checkResult(t, src)
}

func TestNestedBlocks(t *testing.T) {
	src := `Die Zahl z ist 42.
Wenn z größer als 10 ist, dann:
	Für jede Zahl n von 1 bis 3, mache:
		Wenn n gleich 2 ist, dann:
			Erhöhe z um n.
		Sonst:
			Verringere z um n.`
	setTest(t, src)
	checkResult(t, src)
}
