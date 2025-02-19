package eval_test

import (
	"os"
	"testing"
	"time"

	"src.elv.sh/pkg/diag"
	. "src.elv.sh/pkg/eval"
	"src.elv.sh/pkg/parse"

	"src.elv.sh/pkg/eval/errs"
	. "src.elv.sh/pkg/eval/evaltest"
	"src.elv.sh/pkg/eval/vals"
	"src.elv.sh/pkg/testutil"
)

func TestKindOf(t *testing.T) {
	Test(t,
		That("kind-of a []").Puts("string", "list"),
		thatOutputErrorIsBubbled("kind-of a"),
	)
}

func TestConstantly(t *testing.T) {
	Test(t,
		That(`var f = (constantly foo); $f; $f`).Puts("foo", "foo"),
		thatOutputErrorIsBubbled("(constantly foo)"),
	)
}

func TestCallCommand(t *testing.T) {
	Test(t,
		That(`call {|arg &opt=v| put $arg $opt } [foo] [&opt=bar]`).
			Puts("foo", "bar"),
		That(`call { } [foo] [&[]=bar]`).
			Throws(errs.BadValue{What: "option key", Valid: "string", Actual: "list"}),
	)
}

func TestEval(t *testing.T) {
	Test(t,
		That("eval 'put x'").Puts("x"),
		// Using variable from the local scope.
		That("var x = foo; eval 'put $x'").Puts("foo"),
		// Setting a variable in the local scope.
		That("var x = foo; eval 'set x = bar'; put $x").Puts("bar"),
		// Using variable from the upvalue scope.
		That("var x = foo; { nop $x; eval 'put $x' }").Puts("foo"),
		// Specifying a namespace.
		That("var n = (ns [&x=foo]); eval 'put $x' &ns=$n").Puts("foo"),
		// Altering variables in the specified namespace.
		That("var n = (ns [&x=foo]); eval 'set x = bar' &ns=$n; put $n[x]").Puts("bar"),
		// Newly created variables do not appear in the local namespace.
		That("eval 'x = foo'; put $x").DoesNotCompile(),
		// Newly created variables do not alter the specified namespace, either.
		That("var n = (ns [&]); eval &ns=$n 'var x = foo'; put $n[x]").
			Throws(vals.NoSuchKey("x"), "$n[x]"),
		// However, newly created variable can be accessed in the final
		// namespace using &on-end.
		That("eval &on-end={|n| put $n[x] } 'var x = foo'").Puts("foo"),
		// Parse error.
		That("eval '['").Throws(ErrorWithType(&parse.Error{})),
		// Compilation error.
		That("eval 'put $x'").Throws(ErrorWithType(&diag.Error{})),
		// Exception.
		That("eval 'fail x'").Throws(FailError{"x"}),
	)
}

func TestDeprecate(t *testing.T) {
	Test(t,
		That("deprecate msg").PrintsStderrWith("msg"),
		// Different call sites trigger multiple deprecation messages
		That("fn f { deprecate msg }", "f 2>"+os.DevNull, "f").
			PrintsStderrWith("msg"),
		// The same call site only triggers the message once
		That("fn f { deprecate msg}", "fn g { f }", "g 2>"+os.DevNull, "g 2>&1").
			DoesNothing(),
	)
}

func TestTime(t *testing.T) {
	Test(t,
		// Since runtime duration is non-deterministic, we only have some sanity
		// checks here.
		That("time { echo foo } | var a _ = (all)", "put $a").Puts("foo"),
		That("var duration = ''",
			"time &on-end={|x| set duration = $x } { echo foo } | var out = (all)",
			"put $out", "kind-of $duration").Puts("foo", "number"),
		That("time { fail body } | nop (all)").Throws(FailError{"body"}),
		That("time &on-end={|_| fail on-end } { }").Throws(
			FailError{"on-end"}),

		That("time &on-end={|_| fail on-end } { fail body }").Throws(
			FailError{"body"}),

		thatOutputErrorIsBubbled("time { }"),
	)
}

func TestUseMod(t *testing.T) {
	testutil.InTempDir(t)
	testutil.MustWriteFile("mod.elv", "var x = value")

	Test(t,
		That("put (use-mod ./mod)[x]").Puts("value"),
	)
}

func timeAfterMock(fm *Frame, d time.Duration) <-chan time.Time {
	fm.ValueOutput().Put(d) // report to the test framework the duration we received
	return time.After(0)
}

func TestSleep(t *testing.T) {
	TimeAfter = timeAfterMock
	Test(t,
		That(`sleep 0`).Puts(0*time.Second),
		That(`sleep 1`).Puts(1*time.Second),
		That(`sleep 1.3s`).Puts(1300*time.Millisecond),
		That(`sleep 0.1`).Puts(100*time.Millisecond),
		That(`sleep 0.1ms`).Puts(100*time.Microsecond),
		That(`sleep 3h5m7s`).Puts((3*3600+5*60+7)*time.Second),

		That(`sleep 1x`).Throws(ErrInvalidSleepDuration, "sleep 1x"),
		That(`sleep -7`).Throws(ErrNegativeSleepDuration, "sleep -7"),
		That(`sleep -3h`).Throws(ErrNegativeSleepDuration, "sleep -3h"),

		That(`sleep 1/2`).Puts(time.Second/2), // rational number string

		// Verify the correct behavior if a numeric type, rather than a string, is passed to the
		// command.
		That(`sleep (num 42)`).Puts(42*time.Second),
		That(`sleep (float64 0)`).Puts(0*time.Second),
		That(`sleep (float64 1.7)`).Puts(1700*time.Millisecond),
		That(`sleep (float64 -7)`).Throws(ErrNegativeSleepDuration, "sleep (float64 -7)"),

		// An invalid argument type should raise an exception.
		That(`sleep [1]`).Throws(ErrInvalidSleepDuration, "sleep [1]"),
	)
}

func TestResolve(t *testing.T) {
	libdir := testutil.InTempDir(t)
	testutil.MustWriteFile("mod.elv", "fn func { }")

	TestWithSetup(t, func(ev *Evaler) { ev.LibDirs = []string{libdir} },
		That("resolve for").Puts("special"),
		That("resolve put").Puts("$put~"),
		That("fn f { }; resolve f").Puts("$f~"),
		That("use mod; resolve mod:func").Puts("$mod:func~"),
		That("resolve cat").Puts("(external cat)"),
		That(`resolve external`).Puts("$external~"),
	)
}
