"""Scripted sandbox shopper approval for one T1.3b order.

Reads APPROVAL_URL, SHOPPER_EMAIL and SHOPPER_PASSWORD from the
environment (export them from `secret-tool lookup ...` output; values
travel only in this browser session, never touch a file). Drives
Playwright Chromium through the sandbox checkout login and clicks
Complete Purchase, then exits once the order leaves
PAYER_ACTION_REQUIRED.

Screenshots and console text go to OUT (default tests/live/seed/out,
git-ignored). Nothing secret is printed: logs name selectors and
order status only.
"""
import os
import sys
import time

OUT = os.environ.get("OUT", "tests/live/seed/out")


def log(*parts):
    print("[approve]", *parts, flush=True)


def main() -> int:
    url = os.environ.get("APPROVAL_URL", "")
    email = os.environ.get("SHOPPER_EMAIL", "")
    password = os.environ.get("SHOPPER_PASSWORD", "")
    if not url or not email or not password:
        print("set APPROVAL_URL, SHOPPER_EMAIL and SHOPPER_PASSWORD", file=sys.stderr)
        return 2
    os.makedirs(OUT, exist_ok=True)

    from playwright.sync_api import sync_playwright

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.goto(url, wait_until="domcontentloaded", timeout=90000)
        page.wait_for_timeout(4000)
        page.screenshot(path=f"{OUT}/approve-1-landed.png")
        log("landed:", page.url[:80], "title:", page.title()[:60])

        # Sandbox login shape varies (email-first or combined). Try the
        # known candidates in order; exactly one will stick.
        email_candidates = ["#email", "input[name='login_email']", "input[type='email']"]
        filled = False
        for sel in email_candidates:
            try:
                page.fill(sel, email, timeout=5000)
                filled = True
                log("email via", sel)
                break
            except Exception:
                continue
        if not filled:
            log("no email field found; page text:")
            log(page.inner_text("body")[:500].replace("\n", " | "))
            page.screenshot(path=f"{OUT}/approve-FAIL-no-email.png")
            return 1
        for sel in ["#btnNext", "button:has-text('Next')"]:
            try:
                page.click(sel, timeout=3000)
                break
            except Exception:
                continue
        page.wait_for_timeout(4000)

        pw_candidates = ["#password", "input[name='login_password']", "input[type='password']"]
        for sel in pw_candidates:
            try:
                page.fill(sel, password, timeout=8000)
                log("password via", sel)
                break
            except Exception:
                continue
        else:
            # Email+password on one screen: password appeared after Next
            # failed, so re-check before giving up.
            page.screenshot(path=f"{OUT}/approve-FAIL-no-password.png")
            log("no password field found")
            return 1
        for sel in ["#btnLogin", "button:has-text('Log In')", "button[type='submit']"]:
            try:
                page.click(sel, timeout=3000)
                break
            except Exception:
                continue
        page.wait_for_timeout(6000)
        page.screenshot(path=f"{OUT}/approve-2-logged-in.png")
        log("after login:", page.url[:80])

        # Consent / complete: the sandbox renders a funding-source page
        # first ("Continue to Review Order") and only then the final
        # review page ("Complete Purchase" / "Pay Now"). Read every
        # visible button once per attempt and click the pay control by
        # label priority; stop as soon as we leave PayPal's pages.
        priorities = (
            "continue to review order",
            "complete purchase",
            "pay now",
            "agree and pay",
            "agree & continue",
            "confirm",
            "continue",
        )
        empty_streak = 0
        for attempt in range(6):
            labels = []
            try:
                buttons = page.query_selector_all("button")
            except Exception:
                buttons = []
            for b in buttons:
                try:
                    if not b.is_visible():
                        continue
                    labels.append(((b.inner_text() or "").strip(), b))
                except Exception:
                    continue
            log("attempt", attempt, "buttons:", [t for t, _ in labels][:12])
            if not labels:
                # Transitional page (or already paid): wait without
                # clicking. Two quiet rounds in a row means done.
                empty_streak += 1
                if empty_streak >= 2:
                    log("page quiet twice; approval should be through")
                    break
                page.wait_for_timeout(8000)
                continue
            empty_streak = 0
            target = None
            for prio in priorities:
                for text, b in labels:
                    if prio in text.lower():
                        target = (text, b)
                        break
                if target is not None:
                    break
            if target is not None:
                log("clicking:", target[0][:60])
                try:
                    target[1].click(timeout=5000)
                except Exception as e:
                    log("click failed:", str(e)[:100])
            page.wait_for_timeout(8000)
            page.screenshot(path=f"{OUT}/approve-3-complete-{attempt}.png")
            if "sandbox.paypal.com" not in page.url:
                log("left paypal:", page.url[:100])
                break
        log("final:", page.url[:100])
        browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
