package main

import (
	"context"
)

// type Foo struct {
// 	Name string
// }

type (
	// Bar struct {
	// 	Age int
	// }

	Baz struct {
		Height int
	}
)

// type (
// 	Qux struct {
// 		Width int
// 	}

// 	Quux string
// )

const Length = 10

const (
	PI = 3.14
	/* HEIGHT, */ WIDTH = /* 2.718, */ 3.0
	PREFIX              = /* , SUFFIX */ "prefix" /* , "suffix" */
)

// var version = "1.0.0"

var (
	/* foo, */ foz Foo = /* Foo{Name: "foo"}, */ Foo{Name: "foz"}
	// bar        Bar
	baz  Baz
	str0 = /* , str1 */ "str0" /* , "str1" */
	/* str3, */ str4 string = "extraStr"
	// str5, str6        = "str5", "str6"
)

// func Create() {}

func create(ctx context.Context) *User {
	return hookCreate(func(ctx context.Context) *User {
		return &User{}
	})(ctx)
}

func update() int {
	return hookUpdate(func() int {
		// just update
		return 0
	})()
}

func Delete() (string, error) { return "", nil /* just delete */ }

func get[T, A any](ctx context.Context, v T) (A, error) {
	return hookGet(func(ctx context.Context, v T) (A, error) {
		var nop A
		// just get
		return nop, nil
	})(ctx, v)
}

// List
func List[T, A any](ctx context.Context, x, y T, z A) (A, error) {
	return HookList(func(ctx context.Context, x, y T, z A) (A, error) {
		var nop A
		return nop, nil
		// just list
	})(ctx, x, y, z)
}

func (c Company) GetManager() *Manager {
	return &Manager{}
}

type User struct {
	Name string
}

func (u *User) Create() {}

func (u *User) create(ctx context.Context) *User {
	return &User{}
}

func (u *User) update() int {
	// just update
	return 0
}

func (u *User) Delete() (string, error) {
	return u.HookDelete(func() (string, error) {
		return "", nil /* just delete */
	})()
}

// func (u User[T, A]) get(ctx context.Context, v T) (A, error) {
// 	var nop A
// 	// just get
// 	return nop, nil
// }

// List
func (u *User[T, A]) List(ctx context.Context, x, y T, z A) (A, error) {
	return u.HookList(func(ctx context.Context, x, y T, z A) (A, error) {
		var nop A
		return nop, nil
		// just list
	})(ctx, x, y, z)
}
