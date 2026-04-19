package config

import (
	"errors"
	"net/url"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
	"github.com/bk-bf/kino/internal/jellyfin"
)

const (
	hostField = iota
	usernameField
	passwordField
)

var (
	pinkColor       = lipgloss.Color("#923FAD")
	brightPinkColor = lipgloss.Color("#B266D4")
	textColor       = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ddd"}
	dimTextColor    = lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777"}

	titleStyle = lipgloss.NewStyle().Margin(1, 0, 1, 1).Padding(0, 2).Background(pinkColor)
	labelStyle = lipgloss.NewStyle().Margin(0, 1, 0, 1).Foreground(brightPinkColor)
	inputStyle = lipgloss.NewStyle().Foreground(textColor)
	errStyle   = lipgloss.NewStyle().Margin(0, 0, 0, 1).Foreground(dimTextColor)
)

type setupModel struct {
	client *jellyfin.Client
	err    error
	height int
	width  int
	inputs []textinput.Model
	focus  int
}

func initialSetupModel() setupModel {
	inputs := make([]textinput.Model, 3)

	inputs[hostField] = textinput.New()
	inputs[hostField].Focus()
	inputs[hostField].Prompt = ""
	inputs[hostField].Placeholder = "https://jellyfin.example.com"
	inputs[hostField].SetValue(viper.GetString("host"))
	inputs[hostField].Validate = func(s string) error {
		u, err := url.Parse(s)
		if err != nil {
			return errors.New("invalid format")
		}
		if u.Scheme == "" {
			return errors.New("must include scheme (http:// or https://)")
		}
		if u.Host == "" {
			return errors.New("URL must include host")
		}
		return nil
	}

	inputs[usernameField] = textinput.New()
	inputs[usernameField].Prompt = ""
	inputs[usernameField].Placeholder = "username"
	inputs[usernameField].SetValue(viper.GetString("username"))

	inputs[passwordField] = textinput.New()
	inputs[passwordField].Prompt = ""
	inputs[passwordField].Placeholder = "password"
	inputs[passwordField].EchoMode = textinput.EchoPassword
	inputs[passwordField].SetValue(viper.GetString("password"))

	return setupModel{inputs: inputs}
}

func (m setupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *setupModel) createClient() tea.Cmd {
	host := m.inputs[hostField].Value()
	username := m.inputs[usernameField].Value()
	password := m.inputs[passwordField].Value()
	device := viper.GetString("device")
	deviceID := viper.GetString("device_id")
	clientVersion := viper.GetString("client_version")
	token := viper.GetString("token")
	userID := viper.GetString("user_id")
	return func() tea.Msg {
		client, err := jellyfin.NewClient(host, username, password, device, deviceID, clientVersion, token, userID)
		if err != nil {
			return err
		}
		return client
	}
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case error:
		m.err = msg
		return m, nil

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case *jellyfin.Client:
		viper.Set("host", m.inputs[hostField].Value())
		viper.Set("username", m.inputs[usernameField].Value())
		viper.Set("password", m.inputs[passwordField].Value())
		viper.Set("user_id", msg.UserID)
		viper.Set("token", msg.Token)
		if err := viper.WriteConfig(); err != nil {
			_ = viper.SafeWriteConfig()
		}
		m.client = msg
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.focus == len(m.inputs)-1 {
				valid := m.inputs[hostField].Err == nil && m.inputs[hostField].Value() != "" &&
					m.inputs[usernameField].Value() != ""
				if valid {
					return m, m.createClient()
				}
			}
			m.focus = (m.focus + 1) % len(m.inputs)
		case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
			m.focus--
			if m.focus < 0 {
				m.focus = len(m.inputs) - 1
			}
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			m.focus = (m.focus + 1) % len(m.inputs)
		}
		for i := range m.inputs {
			m.inputs[i].Blur()
		}
		m.inputs[m.focus].Focus()
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m setupModel) View() string {
	labels := []string{"Host", "Username", "Password"}
	sections := []string{titleStyle.Render("jellyfin-tui")}
	for i, lbl := range labels {
		label := labelStyle.Render(lbl)
		inp := inputStyle.Render(m.inputs[i].View())
		errTxt := ""
		if e := m.inputs[i].Err; e != nil {
			errTxt = e.Error()
		}
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, label, inp), errStyle.Render(errTxt))
	}
	if m.err != nil {
		sections = append(sections, errStyle.Render(m.err.Error()))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	content = lipgloss.NewStyle().Width(m.width/2 + 10).Height(m.height / 2).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
