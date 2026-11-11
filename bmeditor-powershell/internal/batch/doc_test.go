package batch

import (
	"reflect"
	"testing"
)

const sample = `@echo off
setlocal enabledelayedexpansion
rem :notalabel in a comment
:: goto nowhere, a comment too
set "name=World"
set /a count=0
set /p answer=Go on?
for %%f in (*.txt) do (
  set /a count+=1
  echo !count! %%~nxf
)
call :greet %name%
if errorlevel 1 goto failed
call build.cmd
goto :EOF

:greet
echo Hello, %~1 from %name% in %~dp0
goto :eof

:failed
echo Failed 1>&2
exit /b 1
:failed
goto missing
`

func TestParse_FindsLabelsJumpsAndVariables(t *testing.T) {
	// Act
	d := Parse(sample)

	// Assert
	var labels []string
	for _, l := range d.Labels {
		labels = append(labels, l.Name)
	}
	if want := []string{"greet", "failed", "failed"}; !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %v, want %v", labels, want)
	}
	var jumps []string
	for _, j := range d.Jumps {
		jumps = append(jumps, j.Name)
	}
	// call build.cmd calls a program, not a label.
	if want := []string{"greet", "failed", "EOF", "eof", "missing"}; !reflect.DeepEqual(jumps, want) {
		t.Errorf("jumps = %v, want %v", jumps, want)
	}
	if want := []string{"answer", "count", "name"}; !reflect.DeepEqual(d.VarNames(), want) {
		t.Errorf("variables = %v, want %v", d.VarNames(), want)
	}
	if _, ok := d.Var("%f"); !ok {
		t.Error("the for loop variable should be a variable")
	}
	var uses []string
	for _, u := range d.Uses {
		uses = append(uses, u.Name)
	}
	if want := []string{"count", "name", "name"}; !reflect.DeepEqual(uses, want) {
		t.Errorf("uses = %v, want %v", uses, want)
	}
	if l, _ := d.Label("GREET"); l.Span != (Span{Line: 16, Start: 1, End: 6}) {
		t.Errorf("greet is at %+v (labels ignore case)", l.Span)
	}
}

func TestCheck_ReportsMissingAndDoubledLabels(t *testing.T) {
	// Act
	problems := Parse(sample).Check()

	// Assert
	if len(problems) != 2 {
		t.Fatalf("problems = %+v", problems)
	}
	if p := problems[0]; p.Severity != 1 || p.Span.Line != 24 || p.Message != "No label :missing for this goto" {
		t.Errorf("first problem = %+v", p)
	}
	if p := problems[1]; p.Severity != 2 || p.Span.Line != 23 {
		t.Errorf("second problem = %+v", p)
	}
}

func TestAt(t *testing.T) {
	d := Parse(sample)
	if at := d.At(11, 8); at.Jump == nil || at.Jump.Name != "greet" {
		t.Errorf("at the call's label: %+v", at)
	}
	if at := d.At(11, 15); at.Use == nil || at.Use.Name != "name" {
		t.Errorf("at %%name%%: %+v", at)
	}
	if at := d.At(17, 2); at.Word != "echo" {
		t.Errorf("at echo: %+v", at)
	}
	def, jumps := d.LabelRefs("greet")
	if def == nil || len(jumps) != 1 {
		t.Errorf("greet's references: %v, %v", def, jumps)
	}
}

func TestSpans_CountUTF16(t *testing.T) {
	d := Parse("echo 😀 %name%")
	if len(d.Uses) != 1 || d.Uses[0].Span.Start != 9 {
		t.Errorf("uses = %+v: the emoji is two UTF-16 units", d.Uses)
	}
	if got := ByteOffset("echo 😀 %name%", 9); got != 11 {
		t.Errorf("ByteOffset = %d, want 11", got)
	}
}

func TestLookup(t *testing.T) {
	for _, n := range []string{"ECHO", "robocopy.exe", "Goto"} {
		if _, ok := Lookup(n); !ok {
			t.Errorf("Lookup(%q) found nothing", n)
		}
	}
	if _, ok := Lookup("nosuchcommand"); ok {
		t.Error("an unknown command should not be found")
	}
}
