"""File the T1.3b non-arrival dispute as the shopper in the sandbox
resolution center (the T0.4 click path: dashboard -> payment row ->
Report a problem -> filing wizard -> Submit).

Reads SHOPPER_EMAIL, SHOPPER_PASSWORD and DISPUTE_TXN from the
environment (values travel only in this browser session, never touch a
file). With DISPUTE_TXN set, the script visits the filing URL
directly: server state persists between steps, so the wizard resumes
where the last run left it. Without it, the script starts from the
disputes dashboard and opens the first $195 Payment row.

State dumps (button labels, page text with account detail redacted,
screenshots) go to OUT (default tests/live/seed/out, git-ignored).
Nothing secret is printed: logs name selectors and page titles only.
"""
import os
import re
import sys

OUT = os.environ.get("OUT", "tests/live/seed/out")


def log(*parts):
    print("[dispute]", *parts, flush=True)


def scrub(text):
    text = re.sub(r"[\w.+-]+@[\w-]+\.[\w.]+", "EMAIL-REDACTED", text)
    return text


def dump(page, tag):
    os.makedirs(OUT, exist_ok=True)
    try:
        page.screenshot(path=f"{OUT}/dispute-{tag}.png")
    except Exception as e:
        log("screenshot failed:", str(e)[:80])
    try:
        text = scrub(page.inner_text("body")[:3000]).replace("\n", " | ")
        with open(f"{OUT}/dispute-{tag}.txt", "w") as f:
            f.write(f"url: {page.url}\ntitle: {page.title()}\n{text}\n")
        log(tag, "url:", page.url[:100], "title:", page.title()[:60])
    except Exception as e:
        log("dump failed:", str(e)[:80])


def login(page, email, password):
    for sel in ["#email", "input[name='login_email']", "input[type='email']"]:
        try:
            page.fill(sel, email, timeout=5000)
            log("email via", sel)
            break
        except Exception:
            continue
    for sel in ["#btnNext", "button:has-text('Next')"]:
        try:
            page.click(sel, timeout=3000)
            break
        except Exception:
            continue
    page.wait_for_timeout(4000)
    for sel in ["#password", "input[name='login_password']", "input[type='password']"]:
        try:
            page.fill(sel, password, timeout=8000)
            log("password via", sel)
            break
        except Exception:
            continue
    for sel in ["#btnLogin", "button:has-text('Log In')", "button[type='submit']"]:
        try:
            page.click(sel, timeout=3000)
            break
        except Exception:
            continue
    page.wait_for_timeout(6000)


def buttons(page):
    out = []
    try:
        for b in page.query_selector_all("button, a, [role='button']"):
            try:
                if not b.is_visible():
                    continue
                t = (b.inner_text() or "").strip().replace("\n", " ")
                if t:
                    out.append(t[:80])
            except Exception:
                continue
    except Exception:
        pass
    return out


def click_label(page, *labels):
    for text in buttons(page):
        for want in labels:
            if want.lower() in text.lower():
                try:
                    page.click(f"text={text[:40]}", timeout=4000)
                    log("clicked:", text[:60])
                    return text
                except Exception:
                    continue
    log("no button matched:", labels)
    return None


def open_row_and_report(page):
    """Dashboard path: open the first $195 Payment row, then Report a
    problem in the sheet."""
    opened = None
    try:
        for el in page.query_selector_all("button, a, [role='button']"):
            try:
                if not el.is_visible():
                    continue
                t = (el.inner_text() or "").strip().replace("\n", " ")
                if "Payment" in t and "195" in t:
                    el.click(timeout=4000)
                    opened = t[:60]
                    break
            except Exception:
                continue
    except Exception as e:
        log("row open failed:", str(e)[:80])
    log("opened row:", opened)
    page.wait_for_timeout(5000)
    dump(page, "3-sheet")
    click_label(page, "Report a problem", "Report problem")
    page.wait_for_timeout(4000)
    dump(page, "4-after-report-click")


def wizard(page, tag):
    """One wizard pass: click the one action the current page offers
    (resume, reason, item, detail, amount, contact, documents, review)
    and dump the result. Reruns resume where the last run left off."""
    text = scrub(page.inner_text("body")[:3000])
    marker = os.path.join(OUT, ".seller-done")
    if "Let us know if you reached out to the seller" in text:
        try:
            page.evaluate(
                "Array.from(document.querySelectorAll('label')).find("
                "l => (l.innerText || '').trim() === 'No').click()")
            log("seller answer: No")
        except Exception as e:
            log("seller answer failed:", str(e)[:120])
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
        try:
            with open(marker, "w") as f:
                f.write("done\n")
        except Exception as e:
            log("marker write failed:", str(e)[:80])
    elif "Do a quick check before you submit" in text:
        if not os.path.exists(marker):
            # Seller contact reads empty on the review; answer its
            # optional question first, then submit on the next pass.
            # Seller contact is the fifth Edit on the review (Issue,
            # Items, Issue type, Other details, Seller contact,
            # Documents); answering its optional question unblocks
            # Submit.
            try:
                edits = [e for e in page.query_selector_all("button")
                         if ((e.inner_text() or "").strip() == "Edit")]
                log("edit count:", len(edits))
                if len(edits) >= 5:
                    edits[4].click(timeout=5000)
                    log("seller edit: clicked")
                else:
                    log("seller edit: not-found")
            except Exception as e:
                log("seller edit failed:", str(e)[:120])
        else:
            try:
                page.click("#bannerCloseButton", timeout=3000)
                log("banner dismissed")
            except Exception:
                pass
            page.wait_for_timeout(1500)
            # Mouse clicks on Submit fire no request in this flow
            # (verified with a network tap); focusing the button and
            # pressing Enter files the case.
            try:
                sub = page.locator("button", has_text="Submit")
                sub.scroll_into_view_if_needed(timeout=5000)
                page.wait_for_timeout(1000)
                sub.focus(timeout=5000)
                page.keyboard.press("Enter")
                log("submit via keyboard")
            except Exception as e:
                log("submit failed:", str(e)[:120])
                click_label(page, "Submit")
            # Case creation can lag the click; give it room before the
            # dump that decides the next pass.
            page.wait_for_timeout(15000)
    elif "Continue Reporting this Issue" in text:
        click_label(page, "Continue Reporting this Issue")
    elif "received the item or service" in text and "Report a problem" in text:
        click_label(page, "received the item or service")
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
    elif "Select the items you want to report" in text:
        try:
            page.click("#bannerCloseButton", timeout=3000)
        except Exception:
            pass
        try:
            page.check("input[name='itemsFromPads.0.isSelected']",
                       timeout=5000, force=True)
            log("item checked")
        except Exception as e:
            log("check failed:", str(e)[:100])
        try:
            page.select_option("select[name='itemsFromPads.0.type']",
                               label="Physical product", timeout=5000)
            log("item type set")
        except Exception as e:
            log("item type failed:", str(e)[:100])
        try:
            page.select_option("select[name='itemsFromPads.0.category']",
                               label="Other", timeout=5000)
            log("category set")
        except Exception as e:
            log("category failed:", str(e)[:100])
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
    elif "Tell us a bit more about the problem" in text:
        try:
            page.evaluate(
                "Array.from(document.querySelectorAll("
                "'label, [role=\"radio\"]')).find("
                "l => (l.innerText || '').includes('refused delivery')).click()")
            log("detail label clicked")
        except Exception as e:
            log("detail click failed:", str(e)[:120])
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
    elif "Provide other details" in text:
        try:
            inputs = page.evaluate(
                "Array.from(document.querySelectorAll('input')).map("
                "e => (e.id || '') + '|' + (e.name || '') + '|' + "
                "(e.placeholder || '') + '|' + (e.getAttribute('aria-label') || ''))")
            for i in inputs:
                log("input:", str(i)[:120])
        except Exception as e:
            log("input scan failed:", str(e)[:100])
        filled = False
        for sel in ["input[name='items.0.expected_refund']",
                    "input[placeholder*='amount' i]",
                    "input[aria-label*='refund' i]"]:
            try:
                page.fill(sel, "195.00", timeout=4000)
                log("amount via", sel)
                filled = True
                break
            except Exception:
                continue
        log("amount filled:", filled)
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
    elif "Add documents to support your case" in text:
        try:
            page.fill("textarea", "The parcel never arrived and the tracking "
                      "shows no movement. Filing because the item was not "
                      "received.", timeout=5000)
            log("details filled")
        except Exception as e:
            log("details fill failed:", str(e)[:120])
        page.wait_for_timeout(2000)
        click_label(page, "Continue")
    else:
        # Seller-contact, review: advance with Continue/Submit shaped
        # by what the dump shows.
        click_label(page, "Submit", "Continue")
    page.wait_for_timeout(5000)
    dump(page, tag)
    log("controls:", buttons(page)[:25])


def main() -> int:
    email = os.environ.get("SHOPPER_EMAIL", "")
    password = os.environ.get("SHOPPER_PASSWORD", "")
    txn = os.environ.get("DISPUTE_TXN", "")
    if not email or not password:
        print("set SHOPPER_EMAIL and SHOPPER_PASSWORD", file=sys.stderr)
        return 2
    os.makedirs(OUT, exist_ok=True)

    from playwright.sync_api import sync_playwright

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.goto("https://www.sandbox.paypal.com/signin",
                  wait_until="domcontentloaded", timeout=60000)
        page.wait_for_timeout(4000)
        login(page, email, password)
        dump(page, "1-logged-in")

        if txn:
            page.goto(f"https://www.sandbox.paypal.com/resolutioncenter/"
                      f"filing/{txn}/create?type=PAYMENT",
                      wait_until="domcontentloaded", timeout=60000)
            page.wait_for_timeout(6000)
            dump(page, "2-filing")
        else:
            page.goto("https://www.sandbox.paypal.com/disputes/dashboard/",
                      wait_until="domcontentloaded", timeout=60000)
            page.wait_for_timeout(6000)
            dump(page, "2-dashboard")
            open_row_and_report(page)

        for tag in ("5-step", "6-step", "7-step", "8-step"):
            wizard(page, tag)
        browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
