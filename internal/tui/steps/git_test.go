package steps

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── CanSwitchTabs ──────────────────────────────────────────────────────────────

func TestGitCanSwitchTabs_FalseWhenFormActive_NameInput(t *testing.T) {
	m := NewGitModel()
	// focusIdx=0 (name input) — form not submitted
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false while the git form is active (name input focused)")
	}
}

func TestGitCanSwitchTabs_FalseWhenFormActive_EmailInput(t *testing.T) {
	m := NewGitModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(*GitModel)
	// focusIdx=1 (email input)
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false while the git form is active (email input focused)")
	}
}

func TestGitCanSwitchTabs_FalseWhenFormActive_GPGToggle(t *testing.T) {
	m := NewGitModel()
	// Advance to focusIdx=2 (GPG toggle)
	for i := 0; i < 2; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(*GitModel)
	}
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false while the git form is active (GPG toggle focused)")
	}
}

func TestGitCanSwitchTabs_FalseWhenFormActive_ContinueButton(t *testing.T) {
	m := NewGitModel()
	// Advance to focusIdx=3 (Continue button)
	for i := 0; i < 3; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(*GitModel)
	}
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false while the git form is active (Continue button focused)")
	}
}

func TestGitCanSwitchTabs_TrueWhenDone(t *testing.T) {
	m := NewGitModel()
	m.done = true
	if !m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be true after the git form has been submitted")
	}
}

// ── CanQuit ────────────────────────────────────────────────────────────────────

func TestGitCanQuit_AlwaysTrue(t *testing.T) {
	m := NewGitModel()
	if !m.CanQuit() {
		t.Error("CanQuit() should always be true for the Git step")
	}
}
