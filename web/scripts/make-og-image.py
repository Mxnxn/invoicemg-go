"""Compose the 1200x630 Open Graph card for the landing page.

Run from the Invoice-mg root:  python scripts/make-og-image.py
Needs Pillow. Writes public/og-image.png, which index.html points at absolutely.

Dark ground rather than light: the card is most often seen inside a WhatsApp thread, where a
dark tile reads as deliberate against both the light and dark chat backgrounds, and it lets the
amber accent carry without competing with the message bubbles around it.

Colours are the app's own - --bg-canvas (#0d0d0d), --text-primary (#f1f5f9),
--text-secondary (#94a3b8) and --brand (#d97706) - so the card and the page it opens agree.
"""
from PIL import Image, ImageDraw, ImageFont

W, H = 1200, 630
GROUND = (13, 13, 13)
INK = (241, 245, 249)
MUTE = (148, 163, 184)
ACCENT = (217, 119, 6)   # --brand / --primary, the landing page's amber
RULE = (34, 34, 34)

FONT_DIR = "src/Views/Invoice/font/"
bold = lambda s: ImageFont.truetype(FONT_DIR + "OS700.ttf", s)
reg = lambda s: ImageFont.truetype(FONT_DIR + "OS400.ttf", s)

img = Image.new("RGB", (W, H), GROUND)
d = ImageDraw.Draw(img)

# Faint 48px grid, echoing the landing hero's GridPattern. Drawn before everything else and
# masked to the top-right so it reads as texture, not as a table.
for x in range(0, W, 48):
    for y in range(0, H, 48):
        fade = max(0.0, 1.0 - ((W - x) / W * 0.7 + (y / H) * 0.9))
        if fade <= 0.02:
            continue
        v = int(13 + fade * 26)
        d.rectangle([x, y, x + 1, y + 1], fill=(v, v, v + 4))

PAD = 84

# Accent rule above the wordmark - the one piece of colour on the card.
d.rectangle([PAD, 150, PAD + 64, 156], fill=ACCENT)

d.text((PAD, 196), "InvoiceMG", font=bold(96), fill=INK)

tagline = ["Invoicing, challans and job tracking", "for print and packaging"]
y = 330
for line in tagline:
    d.text((PAD, y), line, font=reg(42), fill=MUTE)
    y += 58

# Footer rule + the three things the page actually promises, as plain labels rather than
# feature-marketing. Tabular spacing by hand so they sit on one baseline.
d.rectangle([PAD, 512, W - PAD, 513], fill=RULE)
labels = ["GST invoices", "Job lifecycle queue", "Ledgers & Excel exports"]
x = PAD
f = reg(28)
for i, label in enumerate(labels):
    if i:
        d.text((x, 548), "·", font=f, fill=(70, 70, 78))
        x += d.textlength("·", font=f) + 22
    d.text((x, 548), label, font=f, fill=MUTE)
    x += d.textlength(label, font=f) + 22

img.save("public/og-image.png", "PNG", optimize=True)
print("wrote public/og-image.png  %dx%d" % img.size)
