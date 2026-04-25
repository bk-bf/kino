// Package preview provides a Bubbletea component that renders Jellyfin
// poster/backdrop images using Kitty, Sixel, iTerm2, or a plain-text fallback.
package preview

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/image/draw"
)

// Protocol represents the terminal image rendering protocol to use.
type Protocol int

const (
	ProtocolNone   Protocol = iota // text fallback
	ProtocolKitty                  // Kitty graphics protocol
	ProtocolSixel                  // DEC Sixel
	ProtocolITerm2                 // iTerm2 inline images
)

// DetectProtocol auto-detects the best image protocol for the current terminal.
// Call once at startup and cache the result.
func DetectProtocol(override string) Protocol {
	switch strings.ToLower(override) {
	case "kitty":
		return ProtocolKitty
	case "sixel":
		return ProtocolSixel
	case "iterm2":
		return ProtocolITerm2
	case "none":
		return ProtocolNone
	}
	// auto-detect
	if os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" {
		return ProtocolKitty
	}
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		return ProtocolITerm2
	}
	if strings.Contains(os.Getenv("TERM"), "sixel") || strings.Contains(os.Getenv("COLORTERM"), "sixel") {
		return ProtocolSixel
	}
	// Check $TERM for common sixel-capable terminals
	term := os.Getenv("TERM")
	if term == "mlterm" || term == "yaft-256color" || strings.HasPrefix(term, "foot") {
		return ProtocolSixel
	}
	return ProtocolNone
}

// ImageFetchedMsg is sent when a poster image has been downloaded.
type ImageFetchedMsg struct {
	ItemID  string
	ImgData []byte
	Err     error
}

// Model is the preview pane Bubbletea component.
type Model struct {
	width    int
	height   int
	protocol Protocol
	host     string

	// current item metadata
	itemID       string
	title        string
	year         int32
	runtime      string
	rating       float32
	overview     string
	showBackdrop bool

	// image state
	loading bool
	imgData []byte
	spinner spinner.Model

	// cached image escape sequence (regenerated when size or data changes)
	cachedImgEsc   string
	cachedImgW     int
	cachedImgH     int
	cachedItemID   string
	cachedBackdrop bool
	cachedCellCols int // actual terminal cell columns the image occupies
	cachedCellRows int // actual terminal cell rows the image occupies

	// where the image was last drawn on screen (used to erase at the exact position)
	drawnImgCol int
	drawnUp     int

	// clear state: dimensions of the previously-shown image that must be
	// erased before the next image is drawn.
	prevCellCols int
	prevCellRows int
	prevImgCol   int // column the previous image was drawn at
	prevUp       int // up-rows the previous image was drawn at
	needsClear   bool
}

// New creates a new preview Model.
func New(host string, protocol Protocol) Model {
	slog.Info("preview: New", "host", host, "protocol", protocol)
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	return Model{
		host:     host,
		protocol: protocol,
		spinner:  s,
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// SetSize updates the pane dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetItem updates the currently-displayed item and triggers an async image fetch.
func (m *Model) SetItem(id, title string, year int32, runtime string, rating float32, overview string) tea.Cmd {
	slog.Info("preview: SetItem", "id", id, "title", title, "width", m.width, "height", m.height, "protocol", m.protocol)
	m.itemID = id
	m.title = title
	m.year = year
	m.runtime = runtime
	m.rating = rating
	m.overview = overview
	// Save old image dimensions so we can erase them before the new image lands.
	if m.cachedCellCols > 0 || m.cachedCellRows > 0 {
		m.prevCellCols = m.cachedCellCols
		m.prevCellRows = m.cachedCellRows
		m.needsClear = true
	}
	m.imgData = nil
	m.cachedImgEsc = ""
	m.cachedCellCols = 0
	m.cachedCellRows = 0
	if id == "" {
		m.loading = false
		return nil
	}
	m.loading = true
	return m.fetchImage()
}

// ToggleBackdrop switches between poster and backdrop image.
func (m *Model) ToggleBackdrop() tea.Cmd {
	if m.itemID == "" {
		return nil
	}
	m.showBackdrop = !m.showBackdrop
	if m.cachedCellCols > 0 || m.cachedCellRows > 0 {
		m.prevCellCols = m.cachedCellCols
		m.prevCellRows = m.cachedCellRows
		m.needsClear = true
	}
	m.imgData = nil
	m.cachedImgEsc = ""
	m.cachedCellCols = 0
	m.cachedCellRows = 0
	m.loading = true
	return m.fetchImage()
}

func (m Model) imageURL() string {
	if m.showBackdrop {
		return fmt.Sprintf("%s/Items/%s/Images/Backdrop/0?fillWidth=600", m.host, m.itemID)
	}
	return fmt.Sprintf("%s/Items/%s/Images/Primary?fillWidth=400", m.host, m.itemID)
}

func (m Model) fetchImage() tea.Cmd {
	url := m.imageURL()
	itemID := m.itemID
	slog.Info("preview: fetchImage", "url", url)
	return func() tea.Msg {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return ImageFetchedMsg{ItemID: itemID, Err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return ImageFetchedMsg{ItemID: itemID, Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		}
		data, err := io.ReadAll(resp.Body)
		return ImageFetchedMsg{ItemID: itemID, ImgData: data, Err: err}
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case ImageFetchedMsg:
		if msg.ItemID == m.itemID {
			slog.Info("preview: ImageFetchedMsg", "itemID", msg.ItemID, "dataLen", len(msg.ImgData), "err", msg.Err)
			m.loading = false
			m.imgData = msg.ImgData // may be nil on error — handled in View
			m.cachedImgEsc = ""     // invalidate cache
		}
	}
	return m, nil
}

// ImageRows returns the number of terminal rows the image occupies (exported).
func (m Model) ImageRows() int { return m.imageRows() }

// ImageRows returns the number of terminal rows the image area occupies.
// Uses the cached actual cell rows when available; otherwise estimates.
func (m Model) imageRows() int {
	if m.protocol == ProtocolNone || len(m.imgData) == 0 {
		return 0
	}
	if m.cachedCellRows > 0 {
		return m.cachedCellRows
	}
	// Estimate before image is decoded: half the available inner height.
	innerH := max(m.height-4, 1)
	return max(innerH/2, 1)
}

// imageCols returns the number of terminal columns the image area occupies.
func (m Model) imageCols() int {
	return max(m.width-4, 1)
}

// View renders only text content (border + blank placeholder rows + metadata).
// Image escape sequences are intentionally NOT included here — they bypass
// lipgloss via ImageOverlay.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#923FAD")).
		Width(m.width - 2).
		Height(m.height - 2)

	inner := m.renderTextOnly()
	return borderStyle.Render(inner)
}

// renderTextOnly builds the text portion of the pane. Where an image would be
// displayed it emits blank lines as a placeholder so that the border and
// metadata are pushed to the correct position.
func (m Model) renderTextOnly() string {
	innerW := max(m.width-4, 1)
	innerH := max(m.height-4, 1)

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777"})
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B266D4"))

	if m.itemID == "" {
		return lipgloss.NewStyle().Width(innerW).Height(innerH).Render(dimStyle.Render("No selection"))
	}

	if m.loading {
		spinnerLine := m.spinner.View() + " Loading…"
		return lipgloss.NewStyle().Width(innerW).Height(innerH).Render(spinnerLine)
	}

	var sections []string

	// Image placeholder rows (blank lines that the overlay will paint over)
	imgRows := m.imageRows()
	if imgRows > 0 {
		placeholder := strings.Repeat("\n", imgRows-1)
		sections = append(sections, placeholder)
	}

	// Metadata text
	titleLine := titleStyle.Render(truncate(m.title, innerW))
	sections = append(sections, titleLine)

	if m.year > 0 || m.runtime != "" || m.rating > 0 {
		meta := fmt.Sprintf("%d", m.year)
		if m.runtime != "" {
			meta += " · " + m.runtime
		}
		if m.rating > 0 {
			meta += fmt.Sprintf(" · %.1f★", m.rating)
		}
		sections = append(sections, dimStyle.Render(truncate(meta, innerW)))
	}

	if m.overview != "" {
		wrapped := wordWrap(m.overview, innerW)
		lines := strings.Split(wrapped, "\n")
		remaining := innerH - imgRows - 3
		if remaining > 0 && len(lines) > remaining {
			lines = lines[:remaining]
		}
		sections = append(sections, dimStyle.Render(strings.Join(lines, "\n")))
	}

	return lipgloss.NewStyle().Width(innerW).Height(innerH).Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)
}

// ImageOverlay returns the terminal escape sequence that paints (or clears)
// the image at the correct position relative to the bottom of the rendered layout.
//
// It always emits a clear for the previous image when needsClear is set, even
// while a new image is still loading. This prevents stacking when items with
// different proportions are focused in quick succession.
//
// Returns "" only when there is nothing at all to emit (no image and no clear).
func (m *Model) ImageOverlay(totalHeight, listW int) string {
	if m.protocol == ProtocolNone {
		return ""
	}

	imgCol := listW + 2
	up := totalHeight - 2
	if up < 1 {
		up = 1
	}

	// Build clear prefix for the old image (emitted once when item changes).
	var clearPart string
	if m.needsClear {
		clearPart = m.buildClearEsc(m.prevCellCols, m.prevCellRows, imgCol)
		m.needsClear = false
		m.prevCellCols = 0
		m.prevCellRows = 0
	}

	// Build new image escape (empty while loading or on error).
	var imgPart string
	if len(m.imgData) > 0 && !m.loading {
		cols := m.imageCols()
		rows := m.imageRows()
		if cols > 0 && rows > 0 {
			if m.cachedImgEsc == "" ||
				m.cachedImgW != cols ||
				m.cachedImgH != rows ||
				m.cachedItemID != m.itemID ||
				m.cachedBackdrop != m.showBackdrop {
				m.buildImageEsc(cols, rows)
			}
			imgPart = m.cachedImgEsc
		}
	}

	content := clearPart + imgPart
	if content == "" {
		return ""
	}

	// \x1b7 / \x1b8 = DEC save/restore cursor (universally supported)
	// \x1b[nA = cursor up n rows
	// \x1b[nG = cursor to column n (1-based)
	// \x1b[nB = cursor down n rows
	result := fmt.Sprintf("\x1b7\x1b[%dA\x1b[%dG%s\x1b[%dB\x1b8",
		up, imgCol, content, up)
	slog.Info("preview: ImageOverlay", "totalHeight", totalHeight, "listW", listW, "imgCol", imgCol, "up", up, "escLen", len(result))
	return result
}

// buildClearEsc returns an escape sequence that erases the old image area.
// For Kitty this sends the "delete all placements" command.
// For Sixel/iTerm2 this overwrites the image area with spaces.
// The cursor is assumed to already be at (imageRow, imgCol) when this runs,
// and is restored to that position afterwards.
func (m Model) buildClearEsc(prevCols, prevRows, imgCol int) string {
	switch m.protocol {
	case ProtocolKitty:
		// For Kitty: write spaces over the image cells (clears character buffer)
		// then send the "delete all placements" command (clears Kitty image layer).
		// q=2 suppresses the terminal's acknowledgement response.
		var sb strings.Builder
		if prevCols > 0 && prevRows > 0 {
			blank := strings.Repeat(" ", prevCols)
			for row := 0; row < prevRows; row++ {
				sb.WriteString(fmt.Sprintf("\x1b[%dG", imgCol))
				sb.WriteString(blank)
				if row < prevRows-1 {
					sb.WriteString("\x1b[1B")
				}
			}
			if prevRows > 1 {
				sb.WriteString(fmt.Sprintf("\x1b[%dA", prevRows-1))
			}
			sb.WriteString(fmt.Sprintf("\x1b[%dG", imgCol))
		}
		sb.WriteString("\x1b_Gq=2,a=d,d=A\x1b\\")
		return sb.String()
	default:
		if prevCols <= 0 || prevRows <= 0 {
			return ""
		}
		// Overwrite each row of the old image area with spaces.
		blank := strings.Repeat(" ", prevCols)
		var sb strings.Builder
		for row := 0; row < prevRows; row++ {
			sb.WriteString(fmt.Sprintf("\x1b[%dG", imgCol)) // move to imgCol
			sb.WriteString(blank)
			if row < prevRows-1 {
				sb.WriteString("\x1b[1B") // move down one row
			}
		}
		// Restore cursor to the top of the image area.
		if prevRows > 1 {
			sb.WriteString(fmt.Sprintf("\x1b[%dA", prevRows-1))
		}
		sb.WriteString(fmt.Sprintf("\x1b[%dG", imgCol))
		return sb.String()
	}
}

// buildImageEsc decodes, scales, and encodes the image into an escape sequence.
// It stores the result and actual cell dimensions in the model's cache fields.
func (m *Model) buildImageEsc(cols, rows int) {
	m.cachedImgEsc = ""
	m.cachedImgW = cols
	m.cachedImgH = rows
	m.cachedItemID = m.itemID
	m.cachedBackdrop = m.showBackdrop
	m.cachedCellCols = 0
	m.cachedCellRows = 0

	img, _, err := image.Decode(bytes.NewReader(m.imgData))
	if err != nil {
		img, err = jpeg.Decode(bytes.NewReader(m.imgData))
		if err != nil {
			slog.Info("preview: buildImageEsc decode error", "err", err)
			return
		}
	}

	// Scale image to fit the cell area (1 cell ≈ 8×16 px), maintaining aspect ratio.
	targetW := cols * 8
	targetH := rows * 16

	slog.Info("preview: buildImageEsc", "cols", cols, "rows", rows, "targetW", targetW, "targetH", targetH, "protocol", m.protocol)

	srcBounds := img.Bounds()
	srcW := srcBounds.Max.X - srcBounds.Min.X
	srcH := srcBounds.Max.Y - srcBounds.Min.Y
	if srcW == 0 || srcH == 0 {
		return
	}

	scale := float64(targetW) / float64(srcW)
	if sh := float64(targetH) / float64(srcH); sh < scale {
		scale = sh
	}
	newW := max(int(float64(srcW)*scale), 1)
	newH := max(int(float64(srcH)*scale), 1)

	// Compute the ACTUAL cell dimensions the scaled image will occupy.
	// These are used for Kitty/iTerm2 c=/r= params and for the placeholder.
	actualCols := (newW + 7) / 8
	actualRows := (newH + 15) / 16

	dst := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, srcBounds, draw.Over, nil)

	var esc string
	switch m.protocol {
	case ProtocolKitty:
		esc = renderKitty(dst, actualCols, actualRows)
	case ProtocolITerm2:
		esc = renderITerm2(m.imgData, actualCols, actualRows)
	case ProtocolSixel:
		esc = renderSixel(dst)
		// Sixel: pixel-exact rendering, compute actual cell dims
	}

	m.cachedImgEsc = esc
	m.cachedCellCols = actualCols
	m.cachedCellRows = actualRows
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}

func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	var lines []string
	var current strings.Builder
	for _, word := range words {
		if current.Len() == 0 {
			current.WriteString(word)
		} else if current.Len()+1+len(word) <= width {
			current.WriteByte(' ')
			current.WriteString(word)
		} else {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
