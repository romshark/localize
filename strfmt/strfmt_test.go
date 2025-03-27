package strfmt_test

import (
	"runtime"
	"testing"

	"github.com/romshark/localize/strfmt"
	"github.com/stretchr/testify/require"
)

func TestDedent(t *testing.T) {
	t.Parallel()
	f := func(t *testing.T, expect, input string) {
		t.Helper()
		a := strfmt.Dedent(input)
		require.Equal(t, expect, a)
	}

	f(t, "", ``)
	f(t, "foo", `foo`)
	f(t, "foo", ` foo `)
	f(t, "foo\n\tbar", `foo
	bar`)
	f(t, "foo", `
		foo
	`)
	f(t, "foo", `

		foo

`)
	f(t, "foo\nbar", `
		foo
		bar
	`)
	f(t, "foo\nbar", `
		  foo
		  bar
	`)
	f(t, "foo\nbar", `
            foo
            bar
	`)
	f(t, "foo\n\nbar", `
		foo

		bar
	`)
	f(t, "foo\n bar\nbazz", `
		foo
		 bar
		bazz
	`)
	f(t, "foo\n bar\nbazz", `
		foo
		 bar
		bazz
	`)
	f(t, "foo\n\t\t bar\n\t\tbazz", `foo
		 bar
		bazz
`)
}

func BenchmarkDedent(b *testing.B) {
	var s string
	for b.Loop() {
		s = strfmt.Dedent(`
			Lorem ipsum dolor sit amet, consectetur adipiscing elit.
			Quisque ultrices pretium felis quis iaculis.
			Vestibulum eu augue porttitor ex varius dapibus.
			Nullam nulla lorem, rhoncus in risus quis, porta dignissim ligula.
			In at iaculis ipsum, id ornare orci.
			Curabitur vel nisl non ipsum blandit molestie. Nunc odio eros,
			consequat non enim quis, lacinia euismod neque.
			Phasellus vitae nibh ut sapien placerat venenatis.
			Ut elementum, magna ac hendrerit sagittis, ex justo imperdiet nibh,
			porttitor dignissim quam augue a dui. Donec vitae semper sapien,
			eu cursus lacus. Pellentesque vulputate, sem in euismod fermentum,
			nunc nisl tempus quam, at luctus libero dui et dui.
			Suspendisse vel porttitor sapien. Etiam mollis dui quis molestie cursus.
			Nam a est egestas, rhoncus quam eu, auctor eros.
			Aenean cursus laoreet ex eu consectetur. 
		`)
	}
	runtime.KeepAlive(s)
}
