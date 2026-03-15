package main

import "github.com/charmbracelet/lipgloss"

func newTextStyle(hc string) lipgloss.Style {
	c := lipgloss.Color(hc)
	return lipgloss.NewStyle().Foreground(c)
}

func boldedText(hc string) lipgloss.Style {
	c := lipgloss.Color(hc)
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

func underlinedText(hc string) lipgloss.Style {
	c := lipgloss.Color(hc)
	return lipgloss.NewStyle().Foreground(c).Underline(true)
}

func emText(hc string) lipgloss.Style {
	c := lipgloss.Color(hc)
	return lipgloss.NewStyle().Foreground(c).Italic(true)
}

var (
	emptyStyle   = emText("#888888")
	summaryStyle = emptyStyle
	numberStyle  = boldedText("#7D56F4")
	handleStyle  = boldedText("#00ADD8")
	dateStyle    = newTextStyle("#888888")
	likeStyle    = newTextStyle("#FF6B9D")
	textStyle    = newTextStyle("#FFFFFF")
	urlStyle     = underlinedText("#5C9CFF")
	keyStyle     = boldedText("#7D56F4")
	metaStyle    = newTextStyle("#666666")
)

func metaSeparator() string {
	return metaStyle.Render(" · ")
}
