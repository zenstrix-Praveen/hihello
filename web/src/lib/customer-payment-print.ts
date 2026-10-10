export type PaymentReceiptField = { label: string; value: string; direction?: 'ltr' | 'auto' };
export type PaymentReceipt = { branding: string; heading: string; language: string; fields: PaymentReceiptField[] };

const escapeHTML = (value: string) => value.replace(/[&<>"']/g, character => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[character]!));

/** A self-contained document: no dashboard DOM, stylesheets, images, or network requests. */
export function createPaymentReceiptDocument(receipt: PaymentReceipt): string {
  const language = receipt.language === 'ar' ? 'ar' : 'en';
  const rows: string[] = [];
  for (let index = 0; index < receipt.fields.length; index += 2) {
    const cells = receipt.fields.slice(index, index + 2).map(field =>
      `<td><div class="label">${escapeHTML(field.label)}</div><div class="value"><bdi dir="${field.direction === 'ltr' ? 'ltr' : 'auto'}">${escapeHTML(field.value)}</bdi></div></td>`);
    if (cells.length === 1) cells.push('<td></td>');
    rows.push(`<tr>${cells.join('')}</tr>`);
  }
  return `<!doctype html>
<html lang="${language}" dir="${language === 'ar' ? 'rtl' : 'ltr'}">
<head><meta charset="utf-8"><title>${escapeHTML(receipt.heading)}</title>
<style>
@page { size: A4 portrait; margin: 10mm; }
* { box-sizing: border-box; }
html { margin: 0; padding: 0; background: #fff; }
body { width: 190mm; margin: 0; padding: 0; color: #172033; background: #fff; font: 10pt/1.35 Arial, Helvetica, sans-serif; }
.receipt { width: 100%; margin: 0; padding: 0; break-inside: avoid; page-break-inside: avoid; }
header { margin: 0 0 4mm; padding: 0 0 3mm; border-bottom: 0.4mm solid #253858; break-inside: avoid; }
.brand { margin: 0 0 1mm; font-size: 10pt; font-weight: 700; }
h1 { margin: 0; font-size: 19pt; line-height: 1.2; }
table { width: 100%; table-layout: fixed; border-collapse: collapse; }
tr, td { break-inside: avoid; page-break-inside: avoid; }
td { width: 50%; padding: 2.5mm 3mm; border: 0.2mm solid #d6dce5; vertical-align: top; text-align: start; }
.label { margin: 0 0 1mm; color: #4b5563; font-size: 8pt; }
.value { margin: 0; font-weight: 600; white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-word; }
bdi { unicode-bidi: isolate; }
</style></head>
<body><main id="payment-receipt" class="receipt"><header><p class="brand">${escapeHTML(receipt.branding)}</p><h1>${escapeHTML(receipt.heading)}</h1></header><table><tbody>${rows.join('')}</tbody></table></main></body></html>`;
}

/** Keeps the receipt alive until Chrome closes print preview (including cancellation). */
export function printPaymentReceipt(receipt: PaymentReceipt): Promise<void> {
  return new Promise((resolve, reject) => {
    const frame = document.createElement('iframe');
    frame.setAttribute('aria-hidden', 'true');
    frame.tabIndex = -1;
    frame.title = receipt.heading;
    // Off-screen, but laid out at A4 content width so long IDs can be measured accurately.
    Object.assign(frame.style, {position:'fixed',left:'-10000px',top:'0',width:'190mm',height:'277mm',border:'0',pointerEvents:'none'});
    let finished = false;
    let started = false;
    let printWindow: Window | null = null;
    const cleanup = (error?: unknown) => {
      if (finished) return;
      finished = true;
      clearTimeout(loadTimeout);
      frame.removeEventListener('load', onLoad);
      printWindow?.removeEventListener('afterprint', afterPrint);
      window.removeEventListener('pagehide', onPageHide);
      frame.remove();
      if (error) reject(error); else resolve();
    };
    const afterPrint = () => cleanup();
    const onPageHide = () => cleanup();
    const onLoad = async () => {
      if (started || finished) return;
      started = true;
      try {
        printWindow = frame.contentWindow;
        const printDocument = frame.contentDocument;
        if (!printWindow || !printDocument) throw new Error('Could not open payment receipt.');
        await printDocument.fonts.ready;
        if (finished) return;
        const receiptElement = printDocument.getElementById('payment-receipt');
        if (!receiptElement) throw new Error('Could not load payment receipt.');
        const bounds = receiptElement.getBoundingClientRect();
        // 277mm printable height minus 1mm rounding allowance. Zoom preserves every field,
        // unlike clipping/max-height; ordinary receipts retain their original 10pt type.
        const pixelsPerMM = 96 / 25.4;
        const scale = Math.min(1, (276 * pixelsPerMM) / Math.max(bounds.height, 1), (190 * pixelsPerMM) / Math.max(bounds.width, 1));
        printDocument.body.style.zoom = String(scale);
        // Flush the final fitted layout before handing the document to print preview.
        receiptElement.getBoundingClientRect();
        clearTimeout(loadTimeout);
        printWindow.addEventListener('afterprint', afterPrint, {once:true});
        printWindow.focus();
        printWindow.print();
      } catch (error) { cleanup(error); }
    };
    // This watchdog only covers loading, never an open print dialog.
    const loadTimeout = setTimeout(() => cleanup(new Error('Could not load payment receipt.')), 15000);
    window.addEventListener('pagehide', onPageHide, {once:true});
    frame.addEventListener('load', onLoad, {once:true});
    frame.srcdoc = createPaymentReceiptDocument(receipt);
    document.body.appendChild(frame);
  });
}
