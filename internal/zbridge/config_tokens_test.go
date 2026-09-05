package zbridge

import (
    "sync"
    "testing"
)

func TestParseZaiTokens(t *testing.T) {
    cases := []struct {
        name string
        raw  string
        want []string
    }{
        {"single", "tok1", []string{"tok1"}},
        {"comma", "tok1,tok2,tok3", []string{"tok1", "tok2", "tok3"}},
        {"comma and spaces", " tok1 , tok2 ,tok3 ", []string{"tok1", "tok2", "tok3"}},
        {"newlines", "tok1\ntok2\r\ntok3", []string{"tok1", "tok2", "tok3"}},
        {"semicolons", "tok1;tok2", []string{"tok1", "tok2"}},
        {"empty fragments dropped", "tok1,,tok2,", []string{"tok1", "tok2"}},
        {"blank", "   ", nil},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := parseZaiTokens(tc.raw)
            if len(got) != len(tc.want) {
                t.Fatalf("got %v, want %v", got, tc.want)
            }
            for i := range got {
                if got[i] != tc.want[i] {
                    t.Fatalf("got %v, want %v", got, tc.want)
                }
            }
        })
    }
}

func TestNextZaiTokenRotates(t *testing.T) {
    c := &Config{ZaiTokens: []string{"a", "b", "c"}}
    var got []string
    for i := 0; i < 7; i++ {
        got = append(got, c.NextZaiToken())
    }
    want := []string{"a", "b", "c", "a", "b", "c", "a"}
    for i := range want {
        if got[i] != want[i] {
            t.Fatalf("rotation broke at %d: got %v, want %v", i, got, want)
        }
    }
}

func TestNextZaiTokenSingleAndEmpty(t *testing.T) {
    one := &Config{ZaiTokens: []string{"only"}}
    for i := 0; i < 3; i++ {
        if got := one.NextZaiToken(); got != "only" {
            t.Fatalf("single token changed: %q", got)
        }
    }
    // No token configured is guest mode, which the caller detects as "".
    none := &Config{}
    if got := none.NextZaiToken(); got != "" {
        t.Fatalf("want empty token for guest mode, got %q", got)
    }
}

// Sessions are minted concurrently, so the cursor must not hand the same
// index to two goroutines or skip entries.
func TestNextZaiTokenConcurrent(t *testing.T) {
    c := &Config{ZaiTokens: []string{"a", "b", "c", "d"}}
    const perGoroutine, goroutines = 250, 8

    counts := make([]map[string]int, goroutines)
    var wg sync.WaitGroup
    for g := 0; g < goroutines; g++ {
        wg.Add(1)
        go func(g int) {
            defer wg.Done()
            local := map[string]int{}
            for i := 0; i < perGoroutine; i++ {
                local[c.NextZaiToken()]++
            }
            counts[g] = local
        }(g)
    }
    wg.Wait()

    total := map[string]int{}
    for _, local := range counts {
        for tok, n := range local {
            total[tok] += n
        }
    }
    want := perGoroutine * goroutines / len(c.ZaiTokens)
    for _, tok := range c.ZaiTokens {
        if total[tok] != want {
            t.Fatalf("token %q used %d times, want %d (uneven rotation: %v)",
                tok, total[tok], want, total)
        }
    }
}
