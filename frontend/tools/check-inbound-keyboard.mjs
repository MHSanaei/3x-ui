import assert from 'node:assert/strict';
import { chromium, devices } from 'playwright';

const base = process.argv[2] || 'http://127.0.0.1:6006';
const browser = await chromium.launch({ headless: true });

try {
  for (const mobile of [false, true]) {
    const context = await browser.newContext(mobile ? devices['Pixel 7'] : {});
    const page = await context.newPage();
    const requests = [];
    await page.route('**/panel/api/**', async (route) => {
      const request = route.request();
      if (/\/inbounds\/(add|update\/41)$/.test(new URL(request.url()).pathname)) {
        requests.push({ url: new URL(request.url()).pathname, payload: request.postDataJSON() });
      }
      await route.fulfill({
        json: { success: true, msg: '', obj: { id: 41, datepicker: 'gregorian' } },
      });
    });

    async function tabTo(locator) {
      for (let i = 0; i < 100; i++) {
        if (await locator.evaluate((element) => element === document.activeElement)) return;
        await page.keyboard.press('Tab');
        const escaped = await page.evaluate(() => {
          const dialog = document.querySelector('[role="dialog"]');
          return dialog && !dialog.contains(document.activeElement)
            ? document.activeElement.outerHTML
            : '';
        });
        assert.ok(!escaped, `Tab escaped the open inbound dialog: ${escaped.slice(0, 200)}`);
      }
      assert.fail('Control was not reachable with Tab');
    }

    async function activateTab(dialog, name) {
      const tabs = dialog.getByRole('tab');
      const names = await tabs.allTextContents();
      const current = dialog.getByRole('tab', { selected: true });
      const from = names.indexOf(await current.textContent());
      const to = names.findIndex((text) => text.endsWith(name));
      assert.ok(from >= 0 && to >= 0, `Missing named tab ${name}`);
      await tabTo(current);
      for (let i = 0; i < Math.abs(to - from); i++) {
        await page.keyboard.press(to > from ? 'ArrowRight' : 'ArrowLeft');
        await page.keyboard.press('Enter');
      }
      assert.equal(
        await dialog
          .getByRole('tab', { name: new RegExp(`${name}$`) })
          .getAttribute('aria-selected'),
        'true',
      );
    }

    async function selectByKey(locator, key, label) {
      await tabTo(locator);
      if (key.length === 1) await page.keyboard.press(key);
      else await page.keyboard.type(key);
      assert.equal(await locator.evaluate((element) => element.selectedOptions[0].text), label);
      assert.ok(
        await locator.evaluate((element) => element === document.activeElement),
        'Selection lost focus',
      );
    }

    for (const edit of [false, true]) {
      await page.goto(
        `${base}/iframe.html?id=inbounds-inboundformmodal--${edit ? 'edit' : 'create'}&viewMode=story`,
      );
      const opener = page.getByRole('button', {
        name: edit ? 'Edit inbound' : 'Add inbound',
        exact: true,
      });
      await opener.waitFor();
      await tabTo(opener);
      await page.keyboard.press('Enter');
      const dialog = page.getByRole('dialog', { name: edit ? 'Modify Inbound' : 'Add Inbound' });
      await dialog.waitFor();
      await page.waitForFunction(
        () =>
          !document
            .querySelector('[role="dialog"]')
            ?.getAnimations()
            .some((animation) => animation.playState === 'running'),
      );
      const protocol = dialog.getByRole('combobox', { name: 'Protocol', exact: true });
      if (edit) assert.ok(await protocol.isDisabled());
      else {
        await activateTab(dialog, 'Security');
        const group = dialog.getByRole('radiogroup', { name: 'Security' });
        await tabTo(group.getByRole('radio', { name: 'None' }));
        await page.keyboard.press('ArrowRight');
        assert.ok(await group.getByRole('radio', { name: 'TLS' }).isChecked());
        await page.keyboard.press('ArrowRight');
        assert.ok(await group.getByRole('radio', { name: 'Reality' }).isChecked());
        await page.keyboard.press('ArrowLeft');
        await page.keyboard.press('ArrowLeft');
        assert.ok(await group.getByRole('radio', { name: 'None' }).isChecked());
        await activateTab(dialog, 'Basics');
        if (!mobile) {
          await tabTo(protocol);
          await page.keyboard.press('Alt+ArrowDown');
          await page.keyboard.press('Escape');
          assert.ok(await dialog.isVisible(), 'Canceling a native picker closed the dialog');
        }
        await selectByKey(protocol, 's', 'shadowsocks');
        await activateTab(dialog, 'Protocol');
        await selectByKey(
          dialog.getByRole('combobox', { name: 'Encryption method' }),
          'chacha20-ietf-poly1305',
          'chacha20-ietf-poly1305',
        );
      }
      await activateTab(dialog, 'Stream');
      await selectByKey(
        dialog.getByRole('combobox', { name: 'Transmission' }),
        edit ? 'g' : 'w',
        edit ? 'gRPC' : 'WebSocket',
      );
      const cdp = await context.newCDPSession(page);
      const ax = await cdp.send('Accessibility.getFullAXTree');
      const transport = ax.nodes.find(
        (node) => node.role?.value === 'combobox' && node.name?.value.trim() === 'Transmission',
      );
      assert.equal(transport?.value?.value, edit ? 'gRPC' : 'WebSocket');
      const submit = dialog.getByRole('button', {
        name: edit ? 'Save Changes' : 'Create',
        exact: true,
      });
      await tabTo(submit);
      await page.keyboard.press('Enter');
      await dialog.waitFor({ state: 'hidden' });
      await page.waitForFunction(
        () =>
          document.activeElement?.textContent === 'Add inbound' ||
          document.activeElement?.textContent === 'Edit inbound',
      );
      const saved = requests.at(-1);
      assert.equal(saved?.url, edit ? '/panel/api/inbounds/update/41' : '/panel/api/inbounds/add');
      assert.equal(saved.payload.protocol, 'shadowsocks');
      const settings = JSON.parse(saved.payload.settings);
      assert.equal(settings.method, 'chacha20-ietf-poly1305');
      assert.equal(JSON.parse(saved.payload.streamSettings).network, edit ? 'grpc' : 'ws');
      if (edit) assert.ok(!('clients' in settings), 'Editing must preserve server-owned clients');
      console.log(
        `PASS ${mobile ? 'mobile viewport' : 'desktop'} ${edit ? 'edit' : 'create'}: keyboard, AX value, payload, focus return`,
      );
    }
    await context.close();
  }
} finally {
  await browser.close();
}
