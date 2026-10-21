package ui

import (
	"testing"
	"time"

	"garden_cosmos/internal/garden"
	"github.com/gdamore/tcell/v2"
	"strings"
)

func TestCapturedExecutionsCount(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.session = &garden.Session{}
	screen := tcell.NewSimulationScreen("UTF-8")
	u.app.SetScreen(screen).SetRoot(u.pages, true)
	stage := 0
	u.app.SetAfterDrawFunc(func(tcell.Screen) {
		switch {
		case stage == 0:
			stage++
			go u.app.QueueUpdate(func() { u.execute("/bin/true", nil, []string{"query"}) })
		case stage == 1 && u.session.Total == 1:
			stage++
			go u.app.QueueUpdate(func() { u.execute("/bin/false", nil, []string{"tx"}) })
		case stage == 2 && u.session.Total == 2:
			stage++
			go u.app.QueueUpdate(func() { u.app.Stop() })
		}
	})
	done := make(chan error, 1)
	go func() { done <- u.app.Run() }()
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not finish")
	}
	if u.session.Total != 2 || u.session.ExitZero != 1 || u.session.Failed != 1 {
		t.Fatal(u.session)
	}
}

func TestInteractiveResultIntegration(t *testing.T) {
	u := editorFixture(t)
	u.leaveCommand()
	u.session = &garden.Session{}
	r := garden.Result{Stdout: `{"height":42}`, ExitCode: 0, Started: true, Duration: time.Second}
	u.finishInteractive("/bin/echo", []string{"query"}, r)
	if u.raw != r.Stdout || !strings.Contains(u.displayOutput, `"height"`) || u.app.GetFocus() != u.output {
		t.Fatal("interactive result missing")
	}
	if u.session.Total != 1 || u.session.ExitZero != 1 {
		t.Fatal("session double counted", u.session)
	}
	if !strings.Contains(u.output.GetText(true), "stdout/stderr 통합") {
		t.Fatal("merged output not labelled")
	}
}
