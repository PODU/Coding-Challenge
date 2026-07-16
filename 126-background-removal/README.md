# ccbgremove — Background Removal Tool

[Coding Challenge #126](https://codingchallenges.substack.com/p/coding-challenge-126-background-removal).

Remove the background from a photo **locally** — the subject is kept with a
transparent alpha channel and softened edges. No image ever leaves your machine.

- Input: **JPG / PNG** · Output: **PNG with alpha**, same dimensions.
- Default backend is OpenCV **GrabCut** — no model download, works offline.
- Optional neural backend (**U²-Net via `rembg`**) is auto-used when installed
  for better hair edges.

## Install

```bash
cd 126-background-removal
pip install -e .                 # core (GrabCut) backend
pip install -e ".[neural]"       # + optional U^2-Net backend
pip install -e ".[test]"         # + pytest
```

## Usage

```bash
ccbgremove photo.jpg                    # -> photo-nobg.png
ccbgremove photo.jpg -o cutout.png      # explicit output name
ccbgremove photos/ -o out/              # recurse a folder, mirror its tree
ccbgremove a.jpg b.png -o out/          # several files into one folder
ccbgremove photo.jpg --feather 4        # softer (hairier) edges
ccbgremove photo.jpg --hard-edges       # crisp 0/255 cut
ccbgremove photos/ --backend rembg      # force the neural backend
```

Run without installing:

```bash
PYTHONPATH=src python -m ccbgremove photo.jpg
```

### Options

| Flag | Meaning |
| ---- | ------- |
| `-o, --output` | Output file (single input) or output directory (batch/dirs). |
| `--backend {auto,grabcut,rembg,none}` | Segmentation backend. `auto` = neural if installed, else GrabCut. |
| `--feather N` | Edge-softening radius in px (default 2). |
| `--hard-edges` | Disable feathering. |
| `--margin F` | GrabCut: border fraction assumed background (default 0.08). |
| `--iterations N` | GrabCut refinement iterations (default 5). |
| `--suffix S` | Output filename suffix (default `-nobg`). |
| `--no-recursive` | Don't descend into subdirectories. |
| `--overwrite` | Allow writing over an identical input path. |

### Exit codes

`0` all files succeeded · `1` a file failed / invalid input · `2` bad arguments.
In batch mode every failure is reported together at the end.

## How it works

```
decode → segment (GrabCut / U²-Net) → mask→alpha (+feather) → compose RGBA → PNG
```

## Verify the output

```bash
# alpha channel present + dimensions preserved
python -c "from PIL import Image; im=Image.open('photo-nobg.png'); print(im.mode, im.size)"

# composite onto a contrasting background to eyeball edge quality
python -c "from PIL import Image; fg=Image.open('photo-nobg.png'); \
bg=Image.new('RGBA',fg.size,(255,0,255,255)); \
Image.alpha_composite(bg,fg).save('check.png')"
```

Or with ImageMagick / exiftool:

```bash
magick identify -verbose photo-nobg.png | grep -i alpha
exiftool photo-nobg.png | grep -i "Color Type"
```

## Tests

```bash
pytest        # 29 tests, covering all nine steps
```
