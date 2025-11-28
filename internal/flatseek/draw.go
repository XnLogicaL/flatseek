package flatseek

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/XnLogicaL/flatseek/internal/config"
	"github.com/XnLogicaL/flatseek/internal/util"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// draws input fields on settings form
func (ps *UI) drawSettingsFields(disableAur, disableCache, separateAurCommands, pkgbuildInternal, disableFeed bool) {
	ps.formSettings.Clear(false)
	mode := 0
	if ps.conf.SearchMode != "StartsWith" {
		mode = 1
	}
	by := 0
	if ps.conf.SearchBy != "Name" {
		by = 1
	}
	cIndex := util.IndexOf(config.ColorSchemes(), ps.conf.ColorScheme)
	bIndex := util.IndexOf(config.BorderStyles(), ps.conf.BorderStyle)
	gIndex := util.IndexOf(config.GlyphStyles(), ps.conf.GlyphStyle)

	// handle text/drop-down field changes
	sc := func(txt string) {
		ps.settingsChanged = true
	}

	// input fields
	ps.formSettings.AddDropDown("Color scheme: ", config.ColorSchemes(), cIndex, nil)
	if dd, ok := ps.formSettings.GetFormItemByLabel("Color scheme: ").(*tview.DropDown); ok {
		dd.SetSelectedFunc(func(text string, index int) {
			ps.conf.SetColorScheme(text)
			if cb, ok := ps.formSettings.GetFormItemByLabel("Transparent: ").(*tview.Checkbox); ok {
				ps.conf.SetTransparency(cb.IsChecked())
			}
			ps.applyColors()
			if text != ps.conf.ColorScheme {
				ps.settingsChanged = true
			}
		})
	}
	ps.formSettings.AddCheckbox("Transparent: ", ps.conf.Transparent, func(checked bool) {
		ps.settingsChanged = true
		ps.conf.SetTransparency(checked)
		ps.applyColors()
	})
	ps.formSettings.AddDropDown("Border style: ", config.BorderStyles(), bIndex, nil)
	if dd, ok := ps.formSettings.GetFormItemByLabel("Border style: ").(*tview.DropDown); ok {
		dd.SetSelectedFunc(func(text string, index int) {
			ps.conf.SetBorderStyle(text)
			if text != ps.conf.BorderStyle {
				ps.settingsChanged = true
			}
		})
	}
	ps.formSettings.AddDropDown("Glyph style: ", config.GlyphStyles(), gIndex, nil)
	if dd, ok := ps.formSettings.GetFormItemByLabel("Glyph style: ").(*tview.DropDown); ok {
		dd.SetSelectedFunc(func(text string, index int) {
			ps.conf.SetGlyphStyle(text)
			ps.applyGlyphStyle()
			if text != ps.conf.GlyphStyle {
				ps.settingsChanged = true
			}
		})
	}
	ps.formSettings.AddCheckbox("Save window layout: ", ps.conf.SaveWindowLayout, func(checked bool) {
		ps.settingsChanged = true
	})
	ps.formSettings.AddInputField("Max search results: ", strconv.Itoa(ps.conf.MaxResults), 6, nil, sc).
		AddDropDown("Search mode: ", []string{"StartsWith", "Contains"}, mode, func(text string, index int) {
			if text != ps.conf.SearchMode {
				ps.settingsChanged = true
			}
		}).
		AddDropDown("Search by: ", []string{"Name", "Name & Description"}, by, func(text string, index int) {
			if text != ps.conf.SearchBy {
				ps.settingsChanged = true
			}
		}).
		AddCheckbox("Enable Auto-suggest: ", ps.conf.EnableAutoSuggest, func(checked bool) {
			ps.settingsChanged = true
		}).
		AddCheckbox("Compute \"Required by\": ", ps.conf.ComputeRequiredBy, func(checked bool) {
			ps.settingsChanged = true
		})
	ps.formSettings.AddInputField("Install command: ", ps.conf.InstallCommand, 40, nil, sc).
		AddInputField("Upgrade command: ", ps.conf.SysUpgradeCommand, 40, nil, sc).
		AddInputField("Uninstall command: ", ps.conf.UninstallCommand, 40, nil, sc)

	ps.formSettings.AddInputField("Package column width: ", strconv.Itoa(ps.conf.PackageColumnWidth), 6, nil, func(text string) {
		ps.settingsChanged = true
		width, _ := strconv.Atoi(text)
		ps.drawPackageListContent(ps.shownPackages, width)
	})
	ps.formSettings.AddCheckbox("Separate Deps with Newline: ", ps.conf.SepDepsWithNewLine, func(checked bool) {
		ps.settingsChanged = true
	})

	ps.applyDropDownColors()

	// key bindings
	ps.formSettings.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// CTRL + Left navigates to the previous control
		if event.Key() == tcell.KeyLeft && event.Modifiers() == tcell.ModCtrl {
			if ps.prevComponent != nil {
				ps.app.SetFocus(ps.prevComponent)
			} else {
				ps.app.SetFocus(ps.tablePackages)
			}
			return nil
		}
		// Down / Up / TAB for form navigation
		if event.Key() == tcell.KeyDown ||
			event.Key() == tcell.KeyUp ||
			event.Key() == tcell.KeyTab {
			i, b := ps.formSettings.GetFocusedItemIndex()
			if b > -1 {
				i = ps.formSettings.GetFormItemCount() + b
			}
			n := i
			if event.Key() == tcell.KeyUp {
				n-- // move up
			} else {
				n++ // move down
			}
			if i >= 0 && i < ps.formSettings.GetFormItemCount() {
				// drop downs are excluded from Up / Down handling
				if _, ok := ps.formSettings.GetFormItem(i).(*tview.DropDown); ok {
					if event.Key() != tcell.KeyTAB && event.Modifiers() != tcell.ModCtrl {
						return event
					}
				}
			}
			// Leave settings from
			if b == ps.formSettings.GetButtonCount()-1 && event.Key() != tcell.KeyUp {
				ps.app.SetFocus(ps.inputSearch)
				return nil
			}
			if i == 0 && event.Key() == tcell.KeyUp {
				ps.app.SetFocus(ps.tablePackages)
				return nil
			}
			ps.formSettings.SetFocus(n)
			ps.app.SetFocus(ps.formSettings)
			return nil
		}
		return event
	})
}

// draw package information on screen
func (ps *UI) drawPackageInfo(pkg Package, width int) {
	// remove "Latest news" if they were shown previously
	if ps.flexRight.GetItemCount() == 2 {
		ps.flexRight.RemoveItem(ps.flexRight.GetItem(1))
	}

	// clear content and set name
	ps.tableDetails.Clear().
		SetTitle(" [::b]" + ps.conf.Glyphs().Package + pkg.AppID + " ")
	r := 0
	ln := 0

	fields, order := ps.getDetailFields(pkg)
	maxLen := util.MaxLenMapKey(fields)
	for _, k := range order {
		if v, ok := fields[k]; ok && v != "" {
			if ln == 1 {
				r++
			}
			// split lines if they do not fit on the screen
			w := width - (int(float64(width)*(float64(ps.leftProportion)/10)) + maxLen + 7) // subtract left box, borders, padding and first column
			lines := tview.WordWrap(v, w)
			mr := r
			cell := &tview.TableCell{
				Text:            "[::b]" + k,
				Color:           ps.conf.Colors().Accent,
				BackgroundColor: ps.conf.Colors().DefaultBackground,
			}

			ps.tableDetails.SetCell(r, 0, cell)

			for _, l := range lines {
				if mr != r {
					// we need to add some blank content otherwise it looks weird with some terminal configs
					ps.tableDetails.SetCellSimple(r, 0, "")
				}
				cell := &tview.TableCell{
					Text:            l,
					Color:           tcell.ColorWhite,
					BackgroundColor: ps.conf.Colors().DefaultBackground,
				}
				if k == "Description" {
					cell.SetText("[::b]" + l)
				}
				if k == " Show PKGBUILD" {
					cell.SetText(" " + l).
						SetTextColor(ps.conf.Colors().PackagelistHeader)
				}
				if strings.Contains(k, "URL") {
					cell.SetClickedFunc(func() bool {
						exec.Command("xdg-open", v).Start()
						return true
					})
				}
				if k == "Maintainer" && pkg.Remote == "AUR" {
					cell.SetClickedFunc(func() bool {
						exec.Command("xdg-open", fmt.Sprintf(UrlAurMaintainer, v)).Start()
						return true
					})
				}
				ps.tableDetails.SetCell(r, 1, cell)
				r++
			}
			ln++
			if k == "Package URL" {
				r++
			}
		}
	}
	// check if we got more lines than current screen height
	_, _, _, height := ps.tableDetails.GetInnerRect()
	ps.tableDetailsMore = false
	if r > height-1 {
		ps.tableDetailsMore = true
	}
	ps.tableDetails.ScrollToBeginning()
}

// draw list of upgradable packages
func (ps *UI) drawUpgradable(up []Package, cached bool) {
	ps.tableDetails.Clear().
		SetTitle(" [::b]" + ps.conf.Glyphs().Upgrades + "Upgradable packages ")

	// draw news if enabled
	if !ps.conf.DisableNewsFeed && ps.flexRight.GetItemCount() != 2 {
		ps.flexRight.AddItem(ps.tableNews, ps.conf.FeedMaxItems+4, 0, false)
	}

	// header
	columns := []string{"Package  ", "Remote  ", "New version  ", "Installed version", ""}
	for i, col := range columns {
		hcell := &tview.TableCell{
			Text:            col,
			Color:           ps.conf.Colors().PackagelistHeader,
			BackgroundColor: ps.conf.Colors().DefaultBackground,
		}
		ps.tableDetails.SetCell(0, i, hcell)
	}

	// lines (not ignored)
	r := 1
	for i := 0; i < len(up); i++ {
		r++
		ps.drawUpgradeableLine(up[i], r, false)
	}

	// no updates found message else sysupgrade button
	r += 2
	if len(up) == 0 {
		ps.tableDetails.SetCell(r, 0, &tview.TableCell{
			Text:            "No upgrades found",
			Color:           ps.conf.Colors().PackagelistHeader,
			BackgroundColor: ps.conf.Colors().DefaultBackground,
		})
	} else {
		ps.tableDetails.SetCell(r, 0, &tview.TableCell{
			Text:            " [::b]Sysupgrade",
			Align:           tview.AlignCenter,
			Color:           ps.conf.Colors().SettingsFieldText,
			BackgroundColor: ps.conf.Colors().SearchBar,
			Clicked: func() bool {
				ps.performUpgrade(false)
				ps.cacheInfo.Delete("#upgrades#")
				ps.displayUpgradable()
				return true
			},
		})
	}

	// refresh button
	if cached {
		r += 2
		ps.tableDetails.SetCell(r, 0, &tview.TableCell{
			Text:            " [::b]Refresh",
			Color:           ps.conf.Colors().SettingsFieldText,
			BackgroundColor: ps.conf.Colors().SearchBar,
			Align:           tview.AlignCenter,
			Clicked: func() bool {
				ps.cacheInfo.Delete("#upgrades#")
				ps.displayUpgradable()
				return true
			},
		})
	}

	// set nil to avoid printing package details when resizing
	// somewhat hacky, needs refactoring (well, everything needs refactoring here)
	ps.selectedPackage = nil
}

// draws a line for an upgradable package
func (ps *UI) drawUpgradeableLine(up Package, lNum int, ignored bool) {
	cellDesc := &tview.TableCell{
		Text:            "[::b]" + up.Name,
		Color:           ps.conf.Colors().Accent,
		BackgroundColor: ps.conf.Colors().DefaultBackground,
		Clicked: func() bool {
			ps.selectedPackage = &up
			ps.drawPackageInfo(up, ps.width)
			return true
		},
	}
	cellSource := &tview.TableCell{
		Text:            up.Remote,
		Color:           ps.conf.Colors().Accent,
		BackgroundColor: ps.conf.Colors().DefaultBackground,
	}
	cellVnew := &tview.TableCell{
		Text:            "[::b]" + up.Version,
		Color:           ps.conf.Colors().PackagelistSourceRepository,
		BackgroundColor: ps.conf.Colors().DefaultBackground,
		Clicked: func() bool {
			if ps.conf.ShowPkgbuildInternally {
				ps.selectedPackage = &up
			} else {
				// ps.runCommand(util.Shell(), "-c", ps.getPkgbuildCommand(up.Remote, up.PackageBase))
			}
			return true
		},
	}
	cellVold := &tview.TableCell{
		Text:            up.Version,
		Color:           ps.conf.Colors().PackagelistSourceAUR,
		BackgroundColor: ps.conf.Colors().DefaultBackground,
	}

	ps.tableDetails.SetCell(lNum, 0, cellDesc).
		SetCell(lNum, 1, cellSource).
		SetCell(lNum, 2, cellVnew).
		SetCell(lNum, 3, cellVold)

	// rebuild button for AUR packages
	if up.Remote == "AUR" && !ignored {
		cellRebuild := &tview.TableCell{
			Text:            " [::b]Rebuild / Update",
			Color:           ps.conf.Colors().SettingsFieldText,
			BackgroundColor: ps.conf.Colors().SearchBar,
			Clicked: func() bool {
				ps.installPackage(up, false)
				ps.cacheInfo.Delete("#upgrades#")
				ps.displayUpgradable()
				return true
			},
		}
		ps.tableDetails.SetCell(lNum, 4, cellRebuild)
	}

	if ignored {
		cellDesc.SetTextColor(ps.conf.Colors().PackagelistHeader)
		cellVnew.SetTextColor(ps.conf.Colors().PackagelistHeader)
		cellIgnored := &tview.TableCell{
			Text:            "ignored",
			Color:           ps.conf.Colors().PackagelistHeader,
			BackgroundColor: ps.conf.Colors().DefaultBackground,
		}
		ps.tableDetails.SetCell(lNum, 4, cellIgnored)
	}
}

// draw packages on screen
func (ps *UI) drawPackageListContent(packages []Package, pkgwidth int) {
	ps.tablePackages.Clear()

	// header
	ps.drawPackageListHeader(pkgwidth)

	// rows
	for i, pkg := range packages {
		color := ps.conf.Colors().PackagelistSourceRepository

		if pkg.Remote == "flathub" {
			color = ps.conf.Colors().PackagelistSourceAUR
		}

		ps.tablePackages.SetCell(i+1, 0, &tview.TableCell{
			Text:            pkg.AppID,
			Color:           tcell.ColorWhite,
			BackgroundColor: ps.conf.Colors().DefaultBackground,
			MaxWidth:        pkgwidth,
		}).
			SetCell(i+1, 1, &tview.TableCell{
				Text:            pkg.Version,
				Color:           color,
				BackgroundColor: ps.conf.Colors().DefaultBackground,
			}).
			SetCell(i+1, 2, &tview.TableCell{
				Text:            pkg.Remote,
				Color:           color,
				BackgroundColor: ps.conf.Colors().DefaultBackground,
			}).
			SetCell(i+1, 3, &tview.TableCell{
				Color:       ps.conf.Colors().DefaultBackground,
				Text:        ps.getInstalledStateText(pkg.IsInstalled),
				Expansion:   1000,
				Reference:   pkg.IsInstalled,
				Transparent: true,
			})
	}
	ps.tablePackages.ScrollToBeginning()
}

// adds header row to package table
func (ps *UI) drawPackageListHeader(pkgwidth int) {
	columns := []string{"Package", "Version", "Remote", "Installed"}
	for i, col := range columns {
		col := col
		width := 0
		if i == 0 {
			width = pkgwidth
			col = fmt.Sprintf("%-"+strconv.Itoa(width)+"s", col)
		}
		ps.tablePackages.SetCell(0, i, &tview.TableCell{
			Text:            col,
			NotSelectable:   true,
			Color:           ps.conf.Colors().PackagelistHeader,
			BackgroundColor: ps.conf.Colors().DefaultBackground,
			MaxWidth:        width,
			Clicked: func() bool {
				switch col {
				case "Package":
					ps.sortAndRedrawPackageList('N')
				case "Source":
					ps.sortAndRedrawPackageList('S')
				case "Installed":
					ps.sortAndRedrawPackageList('I')
				}
				return true
			},
		})
	}
}

// sorts and redraws the list of packages
func (ps *UI) sortAndRedrawPackageList(runeKey rune) {
	// n - sort by name
	switch runeKey {
	case 'N': // sort by name
		if ps.sortAscending {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				return ps.shownPackages[i].AppID > ps.shownPackages[j].AppID
			})
		} else {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				return ps.shownPackages[j].AppID > ps.shownPackages[i].AppID
			})
		}
	case 'S': // sort by source
		if ps.sortAscending {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				if ps.shownPackages[i].Remote == ps.shownPackages[j].Remote {
					return ps.shownPackages[j].AppID > ps.shownPackages[i].AppID
				}
				return ps.shownPackages[i].Remote > ps.shownPackages[j].Remote
			})
		} else {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				if ps.shownPackages[i].Remote == ps.shownPackages[j].Remote {
					return ps.shownPackages[j].AppID > ps.shownPackages[i].AppID
				}
				return ps.shownPackages[j].Remote > ps.shownPackages[i].Remote
			})
		}
	case 'I': // sort by installed state
		if ps.sortAscending {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				if ps.shownPackages[i].IsInstalled == ps.shownPackages[j].IsInstalled {
					return ps.shownPackages[j].AppID > ps.shownPackages[i].AppID
				}
				return ps.shownPackages[i].IsInstalled
			})
		} else {
			sort.Slice(ps.shownPackages, func(i, j int) bool {
				if ps.shownPackages[i].IsInstalled == ps.shownPackages[j].IsInstalled {
					return ps.shownPackages[j].AppID > ps.shownPackages[i].AppID
				}
				return ps.shownPackages[j].IsInstalled
			})
		}
	}
	ps.sortAscending = !ps.sortAscending
	ps.drawPackageListContent(ps.shownPackages, ps.conf.PackageColumnWidth)
	ps.tablePackages.Select(1, 0)
}

// composes a map with fields and values (package information) for our details box
func (ps *UI) getDetailFields(pkg Package) (map[string]string, []string) {
	return map[string]string{
			"Name":        pkg.Name,
			"AppID":       pkg.AppID,
			"Description": pkg.Description,
			"Version":     pkg.Version,
			"Branch":      pkg.Branch,
			"Remote":      pkg.Remote,
		}, []string{"Name", "AppID",
			"Description",
			"Version",
			"Branch",
			"Remote"}
}

// join and format different dependencies as string
func getDependenciesJoined(i Package, installedIcon, notInstalledicon string, newline bool) string {
	return ""
}

// updates the "install state" of all packages in cache and package list
func (ps *UI) updateInstalledState() {
	// update cached packages
	sterm := strings.ToLower(ps.inputSearch.GetText())
	cpkg, exp, found := ps.cacheSearch.GetWithExpiration(sterm)
	if found {
		scpkg := cpkg.([]Package)
		for i := 0; i < len(scpkg); i++ {
			scpkg[i].IsInstalled = ps.pkgCheckInstalled(scpkg[i].AppID)
		}
		ps.cacheSearch.Set(sterm, scpkg, time.Until(exp))
	}

	// update currently shown packages
	for i := 1; i < ps.tablePackages.GetRowCount(); i++ {
		isInstalled := ps.pkgCheckInstalled(ps.tablePackages.GetCell(i, 0).Text)
		newCell := &tview.TableCell{
			Text:        ps.getInstalledStateText(isInstalled),
			Expansion:   1000,
			Reference:   isInstalled,
			Transparent: true,
		}
		ps.tablePackages.SetCell(i, 2, newCell)
	}
}

// compose text for "Installed" column in package list
func (ps *UI) getInstalledStateText(isInstalled bool) string {
	glyphs := ps.conf.Glyphs()
	colStrInstalled := "[#ff0000::b]"
	installed := glyphs.NotInstalled

	if isInstalled {
		installed = glyphs.Installed
		colStrInstalled = "[#00ff00::b]"
	}

	if ps.conf.ColorScheme == "Monochrome" || ps.flags.MonochromeMode {
		colStrInstalled = "[white:black:b]"
	}

	whiteBlack := "[white:black:-]"
	if ps.conf.Colors().DefaultBackground == tcell.ColorDefault {
		whiteBlack = "[white:-:-]"
	}
	ret := whiteBlack + glyphs.PrefixState + colStrInstalled + installed + whiteBlack + glyphs.SuffixState

	return ret
}
