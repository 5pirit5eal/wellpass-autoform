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

	log.Printf("[%s] Navigating to Typeform: %s", batch.BatchID, batch.TypeformURL)
	if _, err := page.Goto(batch.TypeformURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return nil, fmt.Errorf("failed to navigate to typeform: %w", err)
	}

	time.Sleep(1 * time.Second)
	takeScreenshot("01_welcome")

	// Step 1: Welcome Screen -> Click "Starten"
	startBtn := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Starten"})
	if err := startBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("welcome button 'Starten' not found: %w", err)
	}
	if err := startBtn.Click(); err != nil {
		return nil, fmt.Errorf("failed to click 'Starten': %w", err)
	}

	// Step 2: Notice Screen -> Click "Weiter"
	time.Sleep(1000 * time.Millisecond)
	weiterBtn := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Weiter"}).Or(page.Locator("button:has-text('Weiter')")).First()
	if err := weiterBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("notice screen button not found: %w", err)
	}
	if err := weiterBtn.Click(); err != nil {
		return nil, fmt.Errorf("failed to dismiss notice screen: %w", err)
	}

	// Step 3: Member Email Address
	time.Sleep(1000 * time.Millisecond)
	emailInput := page.Locator("input[type='email'], input[placeholder*='@']").First()
	if err := emailInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("email input not found: %w", err)
	}
	if err := emailInput.Fill(batch.Email); err != nil {
		return nil, fmt.Errorf("failed to fill email: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := page.Keyboard().Press("Enter"); err != nil {
		return nil, fmt.Errorf("failed to submit email: %w", err)
	}
	takeScreenshot("02_email_filled")

	// Loop through tickets
	for i, ticket := range batch.Tickets {
		ticketNum := i + 1
		log.Printf("[%s] Filling ticket %d/%d: %s on %s.%s.%s (Price: %s)",
			batch.BatchID, ticketNum, len(batch.Tickets), ticket.PoolLabel, ticket.Day, ticket.Month, ticket.Year, ticket.Price)

		// 1. Swimming Pool Dropdown
		time.Sleep(1500 * time.Millisecond)
		dropdownBtn := page.GetByRole("button", playwright.PageGetByRoleOptions{
			Name: "Option eingeben oder aussuchen",
		}).Or(page.Locator("button:has-text('Option eingeben oder aussuchen')")).First()

		if err := dropdownBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
			return nil, fmt.Errorf("ticket %d: dropdown trigger button not found: %w", ticketNum, err)
		}
		if err := dropdownBtn.Click(); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to click dropdown button: %w", ticketNum, err)
		}

		time.Sleep(800 * time.Millisecond)
		searchTerm := extractSearchTerm(ticket.PoolLabel)
		if err := page.Keyboard().Type(searchTerm); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to type search term %q: %w", ticketNum, searchTerm, err)
		}

		time.Sleep(800 * time.Millisecond)
		option := page.GetByRole("option", playwright.PageGetByRoleOptions{
			Name: ticket.PoolLabel,
		}).Or(page.Locator("[role='option']")).First()

		if err := option.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return nil, fmt.Errorf("ticket %d: option %q not found: %w", ticketNum, ticket.PoolLabel, err)
		}
		if err := option.Click(); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to click pool option %q: %w", ticketNum, ticket.PoolLabel, err)
		}

		// 2. Date
		time.Sleep(600 * time.Millisecond)
		tagInput := page.Locator("input[placeholder='TT']").Last()
		monatInput := page.Locator("input[placeholder='MM']").Last()
		jahrInput := page.Locator("input[placeholder='JJJJ']").Last()

		if err := tagInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return nil, fmt.Errorf("ticket %d: date inputs not found: %w", ticketNum, err)
		}
		if err := tagInput.Fill(ticket.Day); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to fill day: %w", ticketNum, err)
		}
		if err := monatInput.Fill(ticket.Month); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to fill month: %w", ticketNum, err)
		}
		if err := jahrInput.Fill(ticket.Year); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to fill year: %w", ticketNum, err)
		}
		if err := jahrInput.Press("Enter"); err != nil {
			_ = page.Locator("button:has-text('Ok')").Last().Click()
		}

		// 3. Ticket Einzelpreis
		time.Sleep(600 * time.Millisecond)
		priceInput := page.Locator("input[placeholder*='Antwort'], input[type='text']").Last()
		if err := priceInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return nil, fmt.Errorf("ticket %d: price input not found: %w", ticketNum, err)
		}
		if err := priceInput.Fill(ticket.Price); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to fill price: %w", ticketNum, err)
		}
		if err := priceInput.Press("Enter"); err != nil {
			_ = page.Locator("button:has-text('Ok')").Last().Click()
		}

		// 4. File Upload
		time.Sleep(1000 * time.Millisecond)
		fileInput := page.Locator("input[type='file']").Last()
		if err := fileInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
			return nil, fmt.Errorf("ticket %d: file upload input not found: %w", ticketNum, err)
		}
		if err := fileInput.SetInputFiles([]string{ticket.FilePath}); err != nil {
			return nil, fmt.Errorf("ticket %d: failed to upload file %s: %w", ticketNum, ticket.FilePath, err)
		}

		// Wait for upload confirmation and click visible Ok button
		time.Sleep(1500 * time.Millisecond)
		okUploadBtn := page.Locator("button:has-text('Ok'):visible, [data-qa*='ok-button']:visible").Last()
		if count, _ := okUploadBtn.Count(); count > 0 {
			if err := okUploadBtn.Click(); err != nil {
				_ = page.Keyboard().Press("Enter")
			}
		} else {
			_ = page.Keyboard().Press("Enter")
		}

		// 5. More tickets prompt? (Only relevant if not the 10th ticket)
		if ticketNum < 10 {
			time.Sleep(1500 * time.Millisecond)
			hasMore := (i < len(batch.Tickets)-1)
			if hasMore {
				jaRadio := page.Locator("[role='radio']:visible:has-text('Ja')").First()
				if count, _ := jaRadio.Count(); count > 0 {
					_ = jaRadio.Click()
				} else {
					_ = page.Keyboard().Press("j")
				}
			} else {
				neinRadio := page.Locator("[role='radio']:visible:has-text('Nein')").First()
				if count, _ := neinRadio.Count(); count > 0 {
					_ = neinRadio.Click()
				} else {
					_ = page.Keyboard().Press("n")
				}
			}
		}

		takeScreenshot(fmt.Sprintf("03_ticket_%d_done", ticketNum))
	}

	// Step 4: Personal Information (Vor- und Nachname)
	time.Sleep(1500 * time.Millisecond)
	nameInput := page.Locator("input:not([type='file']):not([type='radio']):not([type='checkbox']):visible").Last()
	if err := nameInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("name input not found: %w", err)
	}
	if err := nameInput.Fill(batch.FullName); err != nil {
		return nil, fmt.Errorf("failed to fill full name: %w", err)
	}
	takeScreenshot("04_name_filled")
	time.Sleep(300 * time.Millisecond)
	if err := nameInput.Press("Enter"); err != nil {
		_ = page.Keyboard().Press("Enter")
	}

	// Step 5: IBAN
	time.Sleep(1500 * time.Millisecond)
	ibanInput := page.Locator("input:not([type='file']):not([type='radio']):not([type='checkbox']):visible").Last()
	if err := ibanInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("IBAN input not found: %w", err)
	}
	if err := ibanInput.Fill(batch.IBAN); err != nil {
		return nil, fmt.Errorf("failed to fill IBAN: %w", err)
	}
	takeScreenshot("05_iban_filled")
	time.Sleep(300 * time.Millisecond)
	if err := ibanInput.Press("Enter"); err != nil {
		_ = page.Keyboard().Press("Enter")
	}

	// Step 6: BIC
	time.Sleep(1500 * time.Millisecond)
	bicInput := page.Locator("input:not([type='file']):not([type='radio']):not([type='checkbox']):visible").Last()
	if err := bicInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err != nil {
		return nil, fmt.Errorf("BIC input not found: %w", err)
	}
	if err := bicInput.Fill(batch.BIC); err != nil {
		return nil, fmt.Errorf("failed to fill BIC: %w", err)
	}
	takeScreenshot("06_bic_filled")
	time.Sleep(300 * time.Millisecond)
	if err := bicInput.Press("Enter"); err != nil {
		_ = page.Keyboard().Press("Enter")
	}

	// Step 7: Confirmation / Declaration Checkbox/Radio
	time.Sleep(1500 * time.Millisecond)
	declarationRadio := page.Locator("[role='radio']:visible, [role='checkbox']:visible").First()
	if err := declarationRadio.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(15000)}); err == nil {
		_ = declarationRadio.Click(playwright.LocatorClickOptions{Force: playwright.Bool(true)})
	} else {
		_ = page.Keyboard().Press("a")
	}

	time.Sleep(1000 * time.Millisecond)
	takeScreenshot("04_pre_submit_summary")

	// Step 8: Final Submission or Dry Run Protection
	if batch.DryRun {
		log.Printf("[%s] DRY_RUN is TRUE: Form filled successfully. Skipping click on 'Senden'.", batch.BatchID)
		result.Success = true
		return result, nil
	}

	// Production: Click "Antworten übermitteln" / "Senden"
	log.Printf("[%s] Submitting form...", batch.BatchID)
	submitBtn := page.Locator("button:has-text('Senden'), button:has-text('Antworten übermitteln')").Last()
	if err := submitBtn.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(10000)}); err != nil {
		return nil, fmt.Errorf("final submit button not found: %w", err)
	}
	if err := submitBtn.Click(); err != nil {
		return nil, fmt.Errorf("failed to click submit button: %w", err)
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
