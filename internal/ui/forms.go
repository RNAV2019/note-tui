package ui

import (
	"errors"

	"charm.land/huh/v2"
)

var formTheme = huh.ThemeFunc(huh.ThemeCatppuccin)

// runField wraps a single huh field in a themed form.
// ok is false when the user aborted (esc/ctrl+c).
func runField(field huh.Field) (ok bool, err error) {
	form := huh.NewForm(huh.NewGroup(field)).WithTheme(formTheme)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RunSelect shows a huh select (substring-filterable with "/").
// ok is false when cancelled.
func RunSelect(title string, items []string) (choice string, ok bool, err error) {
	sel := huh.NewSelect[string]().
		Title(title).
		Options(huh.NewOptions(items...)...).
		Height(14).
		Value(&choice)
	ok, err = runField(sel)
	return choice, ok, err
}

// RunPrompt shows a single-line input. ok is false when cancelled.
func RunPrompt(label, placeholder string) (value string, ok bool, err error) {
	in := huh.NewInput().
		Title(label).
		Placeholder(placeholder).
		Validate(huh.ValidateNotEmpty()).
		Value(&value)
	ok, err = runField(in)
	return value, ok, err
}

// RunConfirm shows a yes/no dialog defaulting to No.
func RunConfirm(message string) (bool, error) {
	confirmed := false
	c := huh.NewConfirm().
		Title(message).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed)
	ok, err := runField(c)
	if err != nil || !ok {
		return false, err
	}
	return confirmed, nil
}
