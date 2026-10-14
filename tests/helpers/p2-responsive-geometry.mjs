import { expect } from '@playwright/test';

// All checks run against Chromium layout, never jsdom. Vertical document scrolling is allowed.
export async function assertResponsiveGeometry(page, label) {
  await page.evaluate(() => document.fonts.ready);
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const findings = await page.evaluate(() => {
    const tolerance = 2;
    const viewportWidth = document.documentElement.clientWidth;
    const visible = element => {
      const rect = element.getBoundingClientRect();
      const style = getComputedStyle(element);
      return rect.width > 0 && rect.height > 0 && element.checkVisibility({ checkVisibilityCSS: true, contentVisibilityAuto: true }) && style.visibility !== 'hidden' && style.display !== 'none';
    };
    const describe = element => `${element.tagName.toLowerCase()}${element.dataset.testid ? `[${element.dataset.testid}]` : ''}: ${(element.textContent || element.getAttribute('aria-label') || '').trim().slice(0, 70)}`;
    const issues = [];
    if (document.documentElement.scrollWidth > viewportWidth + tolerance || document.body.scrollWidth > viewportWidth + tolerance)
      issues.push(`document horizontal overflow: viewport=${viewportWidth}, html=${document.documentElement.scrollWidth}, body=${document.body.scrollWidth}`);
    const controls = [...document.querySelectorAll('button,input:not([type="hidden"]),select,textarea,summary,a[href]')].filter(visible);
    for (const element of controls) {
      const box = element.getBoundingClientRect();
      if (box.left < -tolerance || box.right > viewportWidth + tolerance)
        issues.push(`control outside viewport: ${describe(element)} [${Math.round(box.left)},${Math.round(box.right)}]`);
      // Native input value scrolling and select popup options are expected; labels/buttons must fit.
      if (element.matches('button,summary,a')) {
        const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
        let text;
        while ((text = walker.nextNode())) {
          if (!text.textContent.trim() || !visible(text.parentElement)) continue;
          const range = document.createRange(); range.selectNodeContents(text);
          for (const rect of range.getClientRects()) {
            if (rect.left < box.left - tolerance || rect.right > box.right + tolerance || rect.top < box.top - tolerance || rect.bottom > box.bottom + tolerance)
              issues.push(`control text clipped: ${describe(element)}`);
          }
        }
      }
    }
    const paintedRect = element => {
      const rect = element.getBoundingClientRect();
      let left = rect.left, right = rect.right, top = rect.top, bottom = rect.bottom;
      for (let parent = element.parentElement; parent && parent !== document.body; parent = parent.parentElement) {
        const style = getComputedStyle(parent), clip = parent.getBoundingClientRect();
        if (['auto', 'scroll', 'hidden', 'clip'].includes(style.overflowX)) {
          left = Math.max(left, clip.left + parent.clientLeft);
          right = Math.min(right, clip.left + parent.clientLeft + parent.clientWidth);
        }
        if (['auto', 'scroll', 'hidden', 'clip'].includes(style.overflowY)) {
          top = Math.max(top, clip.top + parent.clientTop);
          bottom = Math.min(bottom, clip.top + parent.clientTop + parent.clientHeight);
        }
      }
      return { left, right, top, bottom };
    };
    // Only painted peer regions. Off-scroll rows have layout boxes but cannot cover outside controls.
    // Ancestor controls contain their labels and are not collisions.
    for (let a = 0; a < controls.length; a++) for (let b = a + 1; b < controls.length; b++) {
      const first = controls[a], second = controls[b];
      if (first.contains(second) || second.contains(first)) continue;
      const x = paintedRect(first), y = paintedRect(second);
      if (Math.min(x.right, y.right) - Math.max(x.left, y.left) > tolerance && Math.min(x.bottom, y.bottom) - Math.max(x.top, y.top) > tolerance)
        issues.push(`peer controls overlap: ${describe(first)} / ${describe(second)}`);
    }
    for (const element of document.querySelectorAll('h1,h2,h3,p,strong,.error-text,.workspace-identity span')) {
      if (!visible(element) || element.closest('button,summary,option')) continue;
      const style = getComputedStyle(element);
      const box = element.getBoundingClientRect();
      // Truncation of a name is allowed only when the full name remains available through title/label.
      if (style.textOverflow === 'ellipsis' && element.scrollWidth > element.clientWidth + tolerance) {
        const accessibleFullName = element.getAttribute('title') || element.getAttribute('aria-label');
        if (!accessibleFullName || !accessibleFullName.includes(element.textContent.trim())) issues.push(`truncated name has no full-name access: ${describe(element)}`);
        continue;
      }
      const range = document.createRange(); range.selectNodeContents(element);
      for (const rect of range.getClientRects()) {
        if (rect.left < -tolerance || rect.right > viewportWidth + tolerance)
          issues.push(`text outside viewport: ${describe(element)}`);
        if (['hidden', 'clip'].includes(style.overflowX) && (rect.left < box.left - tolerance || rect.right > box.right + tolerance))
          issues.push(`text clipped horizontally: ${describe(element)}`);
        if (['hidden', 'clip'].includes(style.overflowY) && (rect.top < box.top - tolerance || rect.bottom > box.bottom + tolerance))
          issues.push(`text clipped vertically: ${describe(element)}`);
      }
    }
    for (const [a, b] of [['.help-shell', '.app-layout'], ['.video-sidebar', '.workspace'], ['.canvas-area', '.annotation-panel']]) {
      const first = document.querySelector(a), second = document.querySelector(b);
      if (!first || !second || !visible(first) || !visible(second)) continue;
      const x = first.getBoundingClientRect(), y = second.getBoundingClientRect();
      if (Math.min(x.right, y.right) - Math.max(x.left, y.left) > tolerance && Math.min(x.bottom, y.bottom) - Math.max(x.top, y.top) > tolerance)
        issues.push(`supporting panes overlap: ${a} / ${b}`);
    }
    return [...new Set(issues)];
  });
  expect(findings, `${label}: rendered geometry`).toEqual([]);
}

export async function assertReachable(locator) {
  await expect(locator).toBeVisible();
  await locator.scrollIntoViewIfNeeded();
  const reachability = await locator.evaluate(element => {
    const rect = element.getBoundingClientRect();
    const x = Math.min(innerWidth - 1, Math.max(0, rect.x + rect.width / 2));
    const y = Math.min(innerHeight - 1, Math.max(0, rect.y + rect.height / 2));
    const atPoint = document.elementFromPoint(x, y);
    return { horizontallyInside: rect.left >= -2 && rect.right <= innerWidth + 2,
      centerInside: rect.y + rect.height / 2 >= 0 && rect.y + rect.height / 2 <= innerHeight,
      unobscured: !!atPoint && (element === atPoint || element.contains(atPoint)) };
  });
  expect(reachability, `reachable: ${await locator.textContent()}`).toEqual({ horizontallyInside: true, centerInside: true, unobscured: true });
}

export async function doubleTextSize(page) {
  // Doubles every computed font independently, including px overrides. Device scale alone does not enlarge CSS text.
  const rules = await page.evaluate(() => [...document.querySelectorAll('body,body *')].map((element, index) => {
    element.setAttribute('data-p2-text-scale', String(index));
    return `[data-p2-text-scale="${index}"]{font-size:${parseFloat(getComputedStyle(element).fontSize) * 2}px!important}`;
  }).join('\n'));
  await page.addStyleTag({ content: rules });
}

export async function captureResponsiveScene(page, info, name, fixtureState) {
  const viewport = page.viewportSize();
  const filename = `${name}-${viewport.width}x${viewport.height}.png`;
  await page.screenshot({ path: info.outputPath(filename), fullPage: true });
  if (!process.env.P2_SCREENSHOTS || ![320, 1280].includes(viewport.width)) return;
  const { mkdir, copyFile, writeFile } = await import('node:fs/promises');
  const { join } = await import('node:path');
  await mkdir(process.env.P2_SCREENSHOTS, { recursive: true });
  await copyFile(info.outputPath(filename), join(process.env.P2_SCREENSHOTS, filename));
  await writeFile(join(process.env.P2_SCREENSHOTS, `${filename}.json`), JSON.stringify({
    screenshot: filename, fixtureState, synthetic: true, evidenceScope: 'mocked_frontend_ui_only',
    sourceSHA: process.env.P0_EXPECTED_SHA || null, observedAt: new Date().toISOString(),
    browser: page.context().browser().version(), viewport, test: info.title,
  }, null, 2));
}
