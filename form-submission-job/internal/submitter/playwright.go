package submitter

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// PlaywrightSubmitter automates the Typeform web flow using headless Chromium.
type PlaywrightSubmitter struct{}

// NewPlaywrightSubmitter creates a new PlaywrightSubmitter.
func NewPlaywrightSubmitter() *PlaywrightSubmitter {
	return &PlaywrightSubmitter{}
}

// Submit drives the Typeform browser interaction from start to completion.
func (s *PlaywrightSubmitter) Submit(ctx context.Context, batch SubmissionBatch) (*SubmissionResult, error) {
	if len(batch.Tickets) == 0 {
		return nil, fmt.Errorf("no tickets provided for submission")
	}
	if len(batch.Tickets) > 10 {
		return nil, fmt.Errorf("batch exceeds maximum allowed tickets (max 10, got %d)", len(batch.Tickets))
	}

	if batch.ScreenshotsDir != "" {
		if err := os.MkdirAll(batch.ScreenshotsDir, 0755); err != nil {
			log.Printf("Warning: failed to create screenshots directory: %v", err)
		}
	}

	pw, err := playwright.Run()
	if err != nil {
		log.Printf("Playwright driver not found, attempting auto-install: %v", err)
		if installErr := playwright.Install(&playwright.RunOptions{
			Browsers: []string{"chromium"},
		}); installErr != nil {
			return nil, fmt.Errorf("failed to install playwright driver: %w (original error: %v)", installErr, err)
		}
		pw, err = playwright.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to start Playwright after install: %w", err)
		}
	}
	defer func() {
		if stopErr := pw.Stop(); stopErr != nil {
			log.Printf("Warning: error stopping Playwright: %v", stopErr)
		}
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(batch.Headless),
	})
	if err != nil {
		log.Printf("Chromium launch failed (%v), attempting browser install...", err)
		if installErr := playwright.Install(&playwright.RunOptions{
			Browsers: []string{"chromium"},
		}); installErr != nil {
			return nil, fmt.Errorf("failed to install chromium: %w (original error: %v)", installErr, err)
		}
		browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(batch.Headless),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to launch Chromium after install: %w", err)
		}
	}
	defer func() {
		_ = browser.Close()
	}()

	browserContext, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:  &playwright.Size{Width: 1280, Height: 900},
		UserAgent: playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create browser context: %w", err)
	}
	defer func() {
		_ = browserContext.Close()
	}()

	page, err := browserContext.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	result := &SubmissionResult{
		BatchID:          batch.BatchID,
		TicketsSubmitted: len(batch.Tickets),
		Screenshots:      []string{},
	}

	takeScreenshot := func(stepName string) {
		if batch.ScreenshotsDir == "" {
			return
		}
		path := filepath.Join(batch.ScreenshotsDir, fmt.Sprintf("%s_%s_%s.png", batch.BatchID, time.Now().Format("150405"), stepName))
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path: playwright.String(path),
		}); err == nil {
			result.Screenshots = append(result.Screenshots, path)
		}
	}

	failWithScreenshot := func(stageName string, err error) (*SubmissionResult, error) {
		takeScreenshot(fmt.Sprintf("error_%s", stageName))
		return result, err
	}

	log.Printf("[%s] Navigating to Typeform: %s", batch.BatchID, batch.TypeformURL)
	if _, err := page.Goto(batch.TypeformURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return failWithScreenshot("navigation", fmt.Errorf("failed to navigate to typeform: %w", err))
	}

	time.Sleep(1 * time.Second)
	takeScreenshot("01_welcome")

	// Step 1: Welcome Screen -> Click "Starten" or "Beginnen"
	startBtn := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Starten"}).
		Or(page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Beginnen"})).
		Or(page.Locator("button:has-text('Starten')")).
		Or(page.Locator("button:has-text('Beginnen')")).First()
	if err := startBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return failWithScreenshot("welcome_button", fmt.Errorf("welcome button ('Starten' / 'Beginnen') not found: %w", err))
	}
	if err := startBtn.Click(); err != nil {
		return failWithScreenshot("welcome_click", fmt.Errorf("failed to click welcome button: %w", err))
	}

	// Step 2: Notice Screen -> Click "Weiter"
	time.Sleep(1000 * time.Millisecond)
	weiterBtn := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Weiter"}).Or(page.Locator("button:has-text('Weiter')")).First()
	if err := weiterBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return failWithScreenshot("notice_button", fmt.Errorf("notice screen button not found: %w", err))
	}
	if err := weiterBtn.Click(); err != nil {
		return failWithScreenshot("notice_click", fmt.Errorf("failed to dismiss notice screen: %w", err))
	}

	// Step 3: Member Email Address
	if err := waitForFocusedBlockContains(page, "E-Mail", 15000); err != nil {
		time.Sleep(1000 * time.Millisecond)
	}
	emailBlock := page.Locator("[data-qa-focused='true']").First()
	emailInput := emailBlock.Locator("input[type='email'], input[placeholder*='@'], input").First()
	if count, _ := emailInput.Count(); count == 0 {
		emailInput = page.Locator("input[type='email'], input[placeholder*='@']").First()
	}
	if err := emailInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return failWithScreenshot("email_input", fmt.Errorf("email input not found: %w", err))
	}
	if err := emailInput.Fill(batch.Email); err != nil {
		return failWithScreenshot("email_fill", fmt.Errorf("failed to fill email: %w", err))
	}
	time.Sleep(300 * time.Millisecond)
	okEmailBtn := emailBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
	if count, _ := okEmailBtn.Count(); count > 0 {
		_ = okEmailBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	}
	time.Sleep(200 * time.Millisecond)
	if err := page.Keyboard().Press("Enter"); err != nil {
		return failWithScreenshot("email_submit", fmt.Errorf("failed to submit email: %w", err))
	}
	takeScreenshot("02_email_filled")

	// Loop through tickets
	for i, ticket := range batch.Tickets {
		ticketNum := i + 1
		log.Printf("[%s] Filling ticket %d/%d: %s on %s.%s.%s (Price: %s)",
			batch.BatchID, ticketNum, len(batch.Tickets), ticket.PoolLabel, ticket.Day, ticket.Month, ticket.Year, ticket.Price)

		// 1. Swimming Pool Dropdown
		if err := waitForFocusedBlockContains(page, "Schwimmbad", 15000); err != nil {
			time.Sleep(1500 * time.Millisecond)
		}
		poolBlock := page.Locator("[data-qa-focused='true']").First()

		// Check if options are already displayed (e.g. dropdown auto-opened on question entry)
		optionsLocator := page.Locator("[role='option']:visible")
		optsCount, _ := optionsLocator.Count()

		if optsCount == 0 {
			dropdownBtn := poolBlock.Locator("button:has-text('Option eingeben oder aussuchen'), [role='combobox'], [data-qa*='combobox-trigger']").First()
			if count, _ := dropdownBtn.Count(); count == 0 {
				dropdownBtn = page.Locator("button:has-text('Option eingeben oder aussuchen'):visible, [role='combobox']:visible").Last()
			}

			// Wait up to 10s for the trigger button, or check if options appeared
			if err := dropdownBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
				if c, _ := optionsLocator.Count(); c == 0 {
					return failWithScreenshot(fmt.Sprintf("ticket_%d_dropdown_btn", ticketNum), fmt.Errorf("ticket %d: dropdown trigger button not found: %w", ticketNum, err))
				}
			}

			if c, _ := optionsLocator.Count(); c == 0 {
				if err := dropdownBtn.Click(playwright.LocatorClickOptions{
					Force:   playwright.Bool(true),
					Timeout: playwright.Float(10000),
				}); err != nil {
					return failWithScreenshot(fmt.Sprintf("ticket_%d_dropdown_click", ticketNum), fmt.Errorf("ticket %d: failed to click dropdown button: %w", ticketNum, err))
				}
			}
		}

		time.Sleep(600 * time.Millisecond)
		searchTerm := extractSearchTerm(ticket.PoolLabel)
		searchInput := page.Locator("input[role='combobox']:visible, input[placeholder*='Option']:visible, input[placeholder*='aussuchen']:visible, [data-qa*='dropdown'] input:visible").Last()
		if count, _ := searchInput.Count(); count > 0 {
			_ = searchInput.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
			if err := searchInput.Fill(searchTerm); err != nil {
				if seqErr := searchInput.PressSequentially(searchTerm, playwright.LocatorPressSequentiallyOptions{Delay: playwright.Float(40)}); seqErr != nil {
					_ = page.Keyboard().Type(searchTerm)
				}
			}
		} else {
			if err := page.Keyboard().Type(searchTerm); err != nil {
				return failWithScreenshot(fmt.Sprintf("ticket_%d_search_type", ticketNum), fmt.Errorf("ticket %d: failed to type search term %q: %w", ticketNum, searchTerm, err))
			}
		}

		time.Sleep(800 * time.Millisecond)
		option := page.GetByRole("option", playwright.PageGetByRoleOptions{
			Name: ticket.PoolLabel,
		}).Or(page.Locator(fmt.Sprintf("[role='option']:has-text(%q):visible", ticket.PoolLabel))).Or(page.GetByText(ticket.PoolLabel)).First()

		if count, _ := option.Count(); count == 0 {
			option = page.Locator("[role='option']:visible").Filter(playwright.LocatorFilterOptions{
				HasText: ticket.PoolLabel,
			}).First()
			if count, _ := option.Count(); count == 0 {
				option = page.Locator("[role='option']:visible").First()
			}
		}

		if err := option.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_pool_option", ticketNum), fmt.Errorf("ticket %d: option %q not found: %w", ticketNum, ticket.PoolLabel, err))
		}
		if err := option.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)}); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_pool_click", ticketNum), fmt.Errorf("ticket %d: failed to click pool option %q: %w", ticketNum, ticket.PoolLabel, err))
		}

		// 2. Date
		if err := waitForFocusedBlockContains(page, "Besuchsdatum", 10000); err != nil {
			time.Sleep(800 * time.Millisecond)
		}
		dateBlock := page.Locator("[data-qa-focused='true']").First()
		tagInput := dateBlock.GetByPlaceholder("TT").First()
		if count, _ := tagInput.Count(); count == 0 {
			tagInput = page.GetByPlaceholder("TT").Last()
		}
		monatInput := dateBlock.GetByPlaceholder("MM").First()
		if count, _ := monatInput.Count(); count == 0 {
			monatInput = page.GetByPlaceholder("MM").Last()
		}
		jahrInput := dateBlock.GetByPlaceholder("JJJJ").First()
		if count, _ := jahrInput.Count(); count == 0 {
			jahrInput = page.GetByPlaceholder("JJJJ").Last()
		}

		if err := tagInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_date_input", ticketNum), fmt.Errorf("ticket %d: date inputs not found: %w", ticketNum, err))
		}
		_ = tagInput.Click()
		if err := tagInput.Fill(ticket.Day); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_day_fill", ticketNum), fmt.Errorf("ticket %d: failed to fill day: %w", ticketNum, err))
		}
		_ = monatInput.Click()
		if err := monatInput.Fill(ticket.Month); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_month_fill", ticketNum), fmt.Errorf("ticket %d: failed to fill month: %w", ticketNum, err))
		}
		_ = jahrInput.Click()
		if err := jahrInput.Fill(ticket.Year); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_year_fill", ticketNum), fmt.Errorf("ticket %d: failed to fill year: %w", ticketNum, err))
		}
		time.Sleep(300 * time.Millisecond)
		okDateBtn := dateBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
		if count, _ := okDateBtn.Count(); count > 0 {
			_ = okDateBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
		}
		time.Sleep(200 * time.Millisecond)
		_ = page.Keyboard().Press("Enter")

		// 3. Ticket Einzelpreis
		if err := waitForFocusedBlockContains(page, "Ticket-Einzelpreis", 10000); err != nil {
			time.Sleep(800 * time.Millisecond)
		}
		priceBlock := page.Locator("[data-qa-focused='true']").First()
		priceInput := priceBlock.Locator("input").First()
		if count, _ := priceInput.Count(); count == 0 {
			priceInput = page.GetByPlaceholder("Gib hier deine Antwort ein").Or(page.Locator("input[placeholder*='Antwort']:visible")).Last()
		}
		if err := priceInput.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(15000),
			State:   playwright.WaitForSelectorStateVisible,
		}); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_price_input", ticketNum), fmt.Errorf("ticket %d: price input not found: %w", ticketNum, err))
		}

		time.Sleep(400 * time.Millisecond)
		if err := priceInput.Click(); err != nil {
			_ = priceInput.Focus()
		}
		_ = priceInput.Fill("")
		if err := priceInput.PressSequentially(ticket.Price, playwright.LocatorPressSequentiallyOptions{
			Delay: playwright.Float(50),
		}); err != nil {
			if err := priceInput.Fill(ticket.Price); err != nil {
				return failWithScreenshot(fmt.Sprintf("ticket_%d_price_fill", ticketNum), fmt.Errorf("ticket %d: failed to fill price: %w", ticketNum, err))
			}
		}

		time.Sleep(400 * time.Millisecond)
		okPriceBtn := priceBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
		if count, _ := okPriceBtn.Count(); count > 0 {
			_ = okPriceBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
		}
		time.Sleep(200 * time.Millisecond)
		_ = page.Keyboard().Press("Enter")

		// 4. File Upload
		if err := waitForFocusedBlockContains(page, "Rechnungsbeleg hoch", 10000); err != nil {
			time.Sleep(800 * time.Millisecond)
		}
		uploadBlock := page.Locator("[data-qa-focused='true']").First()
		fileInput := uploadBlock.Locator("input[type='file']").First()
		if count, _ := fileInput.Count(); count == 0 {
			fileInput = page.Locator("input[type='file']").Last()
		}
		if err := fileInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
			_ = page.Keyboard().Press("Enter")
			if waitErr := fileInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); waitErr != nil {
				return failWithScreenshot(fmt.Sprintf("ticket_%d_file_input", ticketNum), fmt.Errorf("ticket %d: file upload input not found: %w", ticketNum, waitErr))
			}
		}
		time.Sleep(500 * time.Millisecond)
		if err := fileInput.SetInputFiles([]string{ticket.FilePath}); err != nil {
			return failWithScreenshot(fmt.Sprintf("ticket_%d_file_upload", ticketNum), fmt.Errorf("ticket %d: failed to upload file %s: %w", ticketNum, ticket.FilePath, err))
		}

		// Wait for upload confirmation and click visible Ok button
		time.Sleep(2000 * time.Millisecond)
		okUploadBtn := uploadBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
		if count, _ := okUploadBtn.Count(); count == 0 {
			okUploadBtn = page.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible, button:visible").Filter(playwright.LocatorFilterOptions{
				HasText: "Ok",
			}).Last()
		}

		if err := okUploadBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(20000)}); err == nil {
			_ = okUploadBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
		}
		time.Sleep(500 * time.Millisecond)
		_ = page.Keyboard().Press("Enter")

		takeScreenshot(fmt.Sprintf("03_ticket_%d_done", ticketNum))

		// 5. More tickets prompt? (Only relevant if not the 10th ticket)
		if ticketNum < 10 {
			hasMore := (i < len(batch.Tickets) - 1)

			// Wait until the focused block is the more tickets prompt
			if err := waitForFocusedBlockContains(page, "Willst du noch einen weiteren Rechnungsbeleg hochladen", 15000); err != nil {
				// Retry advancing from upload screen if prompt has not appeared yet
				if count, _ := okUploadBtn.Count(); count > 0 {
					_ = okUploadBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
				}
				_ = page.Keyboard().Press("Enter")
				if retryErr := waitForFocusedBlockContains(page, "Willst du noch einen weiteren Rechnungsbeleg hochladen", 10000); retryErr != nil {
					return failWithScreenshot(fmt.Sprintf("ticket_%d_prompt_wait", ticketNum), fmt.Errorf("ticket %d: more tickets prompt not found: %w", ticketNum, retryErr))
				}
			}

			promptBlock := page.Locator("[data-qa-focused='true']").First()

			if hasMore {
				jaChoice := promptBlock.Locator("[role='radio']:has-text('Ja'), button:has-text('Ja')").First()
				if count, _ := jaChoice.Count(); count == 0 {
					jaChoice = promptBlock.Locator("button[value$='-yes'], [data-qa*='choice']:has-text('Ja')").First()
				}
				if err := jaChoice.WaitFor(playwright.LocatorWaitForOptions{
					Timeout: playwright.Float(10000),
					State:   playwright.WaitForSelectorStateVisible,
				}); err == nil {
					_ = jaChoice.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
				} else {
					_ = page.Keyboard().Press("j")
				}

				// Verify transition away from prompt to the next ticket's dropdown/question
				if err := waitForFocusedBlockNotContains(page, "Willst du noch einen weiteren Rechnungsbeleg hochladen", 2500); err != nil {
					promptOk := promptBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
					if count, _ := promptOk.Count(); count > 0 {
						_ = promptOk.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
					}
					_ = page.Keyboard().Press("Enter")
					if waitErr := waitForFocusedBlockNotContains(page, "Willst du noch einen weiteren Rechnungsbeleg hochladen", 10000); waitErr != nil {
						return failWithScreenshot(fmt.Sprintf("ticket_%d_prompt_advance", ticketNum), fmt.Errorf("ticket %d: failed to advance after selecting 'Ja': %w", ticketNum, waitErr))
					}
				}
			} else {
				neinChoice := promptBlock.Locator("[role='radio']:has-text('Nein'), button:has-text('Nein')").First()
				if count, _ := neinChoice.Count(); count == 0 {
					neinChoice = promptBlock.Locator("button[value$='-no'], [data-qa*='choice']:has-text('Nein')").First()
				}
				if err := neinChoice.WaitFor(playwright.LocatorWaitForOptions{
					Timeout: playwright.Float(10000),
					State:   playwright.WaitForSelectorStateVisible,
				}); err == nil {
					_ = neinChoice.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
				} else {
					_ = page.Keyboard().Press("n")
				}

				// Verify transition away from prompt to personal info (Vor- und Nachname)
				if err := waitForFocusedBlockContains(page, "Vor- und Nachname", 2500); err != nil {
					promptOk := promptBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
					if count, _ := promptOk.Count(); count > 0 {
						_ = promptOk.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
					}
					_ = page.Keyboard().Press("Enter")
					if waitErr := waitForFocusedBlockContains(page, "Vor- und Nachname", 10000); waitErr != nil {
						return failWithScreenshot("prompt_nein_transition", fmt.Errorf("failed to advance to Vor- und Nachname after selecting 'Nein': %w", waitErr))
					}
				}
			}
		}
	}

	// Step 4: Personal Information (Vor- und Nachname)
	if err := waitForFocusedBlockContains(page, "Vor- und Nachname", 15000); err != nil {
		return failWithScreenshot("name_block", fmt.Errorf("personal info block ('Vor- und Nachname') not focused: %w", err))
	}
	nameBlock := page.Locator("[data-qa-focused='true']").First()
	nameInput := nameBlock.Locator("input").First()
	if count, _ := nameInput.Count(); count == 0 {
		nameInput = page.Locator("input[type='short_text'], input:not([type='file']):not([type='radio']):not([type='checkbox']):visible").Last()
	}
	if err := nameInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000), State: playwright.WaitForSelectorStateVisible}); err != nil {
		return failWithScreenshot("name_input", fmt.Errorf("name input not found: %w", err))
	}
	_ = nameInput.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	_ = nameInput.Fill("")
	if err := nameInput.PressSequentially(batch.FullName, playwright.LocatorPressSequentiallyOptions{Delay: playwright.Float(30)}); err != nil {
		if err := nameInput.Fill(batch.FullName); err != nil {
			return failWithScreenshot("name_fill", fmt.Errorf("failed to fill full name: %w", err))
		}
	}
	takeScreenshot("04_name_filled")
	time.Sleep(300 * time.Millisecond)
	okNameBtn := nameBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
	if count, _ := okNameBtn.Count(); count > 0 {
		_ = okNameBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	}
	_ = page.Keyboard().Press("Enter")

	// Step 5: IBAN
	if err := waitForFocusedBlockContains(page, "IBAN", 15000); err != nil {
		return failWithScreenshot("iban_block", fmt.Errorf("IBAN block not focused: %w", err))
	}
	ibanBlock := page.Locator("[data-qa-focused='true']").First()
	ibanInput := ibanBlock.Locator("input").First()
	if count, _ := ibanInput.Count(); count == 0 {
		ibanInput = page.Locator("input[maxlength='27'], input[placeholder*='Antwort']:visible").Last()
	}
	if err := ibanInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000), State: playwright.WaitForSelectorStateVisible}); err != nil {
		return failWithScreenshot("iban_input", fmt.Errorf("IBAN input not found: %w", err))
	}
	_ = ibanInput.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	_ = ibanInput.Fill("")
	if err := ibanInput.PressSequentially(batch.IBAN, playwright.LocatorPressSequentiallyOptions{Delay: playwright.Float(30)}); err != nil {
		if err := ibanInput.Fill(batch.IBAN); err != nil {
			return failWithScreenshot("iban_fill", fmt.Errorf("failed to fill IBAN: %w", err))
		}
	}
	takeScreenshot("05_iban_filled")
	time.Sleep(300 * time.Millisecond)
	okIbanBtn := ibanBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
	if count, _ := okIbanBtn.Count(); count > 0 {
		_ = okIbanBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	}
	_ = page.Keyboard().Press("Enter")

	// Step 6: BIC
	if err := waitForFocusedBlockContains(page, "BIC", 15000); err != nil {
		return failWithScreenshot("bic_block", fmt.Errorf("BIC block not focused: %w", err))
	}
	bicBlock := page.Locator("[data-qa-focused='true']").First()
	bicInput := bicBlock.Locator("input").First()
	if count, _ := bicInput.Count(); count == 0 {
		bicInput = page.Locator("input:not([type='file']):not([type='radio']):not([type='checkbox']):visible").Last()
	}
	if err := bicInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000), State: playwright.WaitForSelectorStateVisible}); err != nil {
		return failWithScreenshot("bic_input", fmt.Errorf("BIC input not found: %w", err))
	}
	_ = bicInput.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	_ = bicInput.Fill("")
	if err := bicInput.PressSequentially(batch.BIC, playwright.LocatorPressSequentiallyOptions{Delay: playwright.Float(30)}); err != nil {
		if err := bicInput.Fill(batch.BIC); err != nil {
			return failWithScreenshot("bic_fill", fmt.Errorf("failed to fill BIC: %w", err))
		}
	}
	takeScreenshot("06_bic_filled")
	time.Sleep(300 * time.Millisecond)
	okBicBtn := bicBlock.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").First()
	if count, _ := okBicBtn.Count(); count > 0 {
		_ = okBicBtn.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	}
	_ = page.Keyboard().Press("Enter")

	// Step 7: Confirmation / Declaration Checkbox/Radio
	if err := waitForFocusedBlockContains(page, "Ich versichere hiermit", 15000); err != nil {
		return failWithScreenshot("decl_block", fmt.Errorf("declaration block not focused: %w", err))
	}
	declBlock := page.Locator("[data-qa-focused='true']").First()
	declRadio := declBlock.Locator("[role='radio'], button:has-text('Ja')").First()
	if count, _ := declRadio.Count(); count == 0 {
		declRadio = page.Locator("[role='radio']:has-text('Ja'):visible, button:has-text('Ja'):not([data-qa*='submit']):visible").First()
	}
	if err := declRadio.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(15000),
		State:   playwright.WaitForSelectorStateVisible,
	}); err == nil {
		_ = declRadio.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	} else {
		_ = page.Keyboard().Press("a")
	}

	time.Sleep(1000 * time.Millisecond)
	takeScreenshot("04_pre_submit_summary")

	// Step 8: Final Submission or Dry Run Protection
	submitBtn := page.Locator("[data-qa*='submit-button']:visible, button:has-text('Einschicken'):visible, button:has-text('Senden'):visible, button:has-text('Antworten übermitteln'):visible").Last()
	if count, _ := submitBtn.Count(); count == 0 {
		submitBtn = page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Einschicken"}).
			Or(page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Senden"})).
			Or(page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Antworten übermitteln"})).Last()
	}
	if err := submitBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return failWithScreenshot("submit_btn", fmt.Errorf("final submit button not found: %w", err))
	}

	if batch.DryRun {
		log.Printf("[%s] DRY_RUN is TRUE: Form filled successfully and submit button verified. Skipping click on submit button ('Einschicken' / 'Senden').", batch.BatchID)
		result.Success = true
		return result, nil
	}

	// Production: Click "Einschicken" / "Senden" / "Antworten übermitteln"
	log.Printf("[%s] Submitting form...", batch.BatchID)
	if err := submitBtn.Click(); err != nil {
		return failWithScreenshot("submit_click", fmt.Errorf("failed to click submit button: %w", err))
	}

	// Wait for thank you / confirmation page
	time.Sleep(3000 * time.Millisecond)
	takeScreenshot("05_post_submit_confirmation")

	result.Success = true
	log.Printf("[%s] Form successfully submitted!", batch.BatchID)
	return result, nil
}

func extractSearchTerm(poolLabel string) string {
	// If label contains quotes or parenthesis, take the most distinct part
	clean := strings.ReplaceAll(poolLabel, "'", "")
	clean = strings.ReplaceAll(clean, "(", " ")
	clean = strings.ReplaceAll(clean, ")", " ")
	words := strings.Fields(clean)
	if len(words) >= 2 {
		return words[0] + " " + words[1]
	}
	if len(words) == 1 {
		return words[0]
	}
	return poolLabel
}

func waitForFocusedBlockContains(page playwright.Page, text string, timeoutMs float64) error {
	_, err := page.WaitForFunction(
		`text => {
			const active = document.querySelector('[data-qa-focused="true"]');
			return active && active.textContent.includes(text);
		}`,
		text,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(timeoutMs)},
	)
	return err
}

func waitForFocusedBlockNotContains(page playwright.Page, text string, timeoutMs float64) error {
	_, err := page.WaitForFunction(
		`text => {
			const active = document.querySelector('[data-qa-focused="true"]');
			return active && !active.textContent.includes(text);
		}`,
		text,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(timeoutMs)},
	)
	return err
}
