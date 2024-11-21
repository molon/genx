package gosurgery

import (
	"context"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	_ "embed"

	"github.com/molon/genx"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/original.genx.go
var originalGenxGo string

func TestBuildIdentMap(t *testing.T) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "", originalGenxGo, parser.ParseComments)
	require.NoError(t, err)

	m := buildIdentMap(af.Decls)
	keys := lo.Keys(m)
	sort.Strings(keys)
	// [Bar Baz Company.GetManager Create Delete Foo HEIGHT Length List PI PREFIX Quux Qux SUFFIX User User.Create User.Delete User.List User.create User.get User.update WIDTH bar baz create foo foz get str0 str1 str3 str4 str5 str6 update version]
	if !assert.Equal(t, []string{
		"Bar", "Baz", "Company.GetManager", "Create", "Delete", "Foo", "HEIGHT", "Length", "List", "PI", "PREFIX", "Quux", "Qux", "SUFFIX", "User", "User.Create", "User.Delete", "User.List", "User.create", "User.get", "User.update", "WIDTH", "bar", "baz", "create", "foo", "foz", "get", "str0", "str1", "str3", "str4", "str5", "str6", "update", "version",
	}, keys) {
		t.Logf("actual result: \n%v", keys)
	}
}

//go:embed testdata/fixed.genx.go
var fixedGenxGo string

func TestSurgery(t *testing.T) {
	userSrc := `
package main

type Foo struct {}

type Bar struct {}
// type Baz struct {}

type Qux string
type Quux struct {}

const HEIGHT = ""
const SUFFIX = ""

var version = "2.0.0"

var foo Foo
var bar Bar
var str1 = "str1x"
var str3 = "str3x"
var str5 = "str5x"
var str6 = "str6x"

func Create() {}
func hookCreate() {}
func hookUpdate(){}
func hookDelete(){} // not func HookDelete(){}
func hookGet[T,A any]() (A, error){}
func HookList[T,A any]() (A, error){}

func (u *User[T, A]) HookDelete() {}
func (u *User[T, A]) get() {}
func (u *User[T, A]) hookGet() {}
func (u User[T, A]) HookList() {}
	`

	generatedFiles := []*genx.File{
		{
			RelPath: "original.genx.go",
			Content: originalGenxGo,
		},
	}
	err := Surgery(context.Background(),
		generatedFiles,
		[]*genx.File{
			{
				RelPath: "user.go",
				Content: userSrc,
			},
		},
	)
	require.NoError(t, err)

	if !assert.Equal(t, fixedGenxGo, generatedFiles[0].Content) {
		t.Logf("actual result: \n%s", generatedFiles[0].Content)
	}
}
