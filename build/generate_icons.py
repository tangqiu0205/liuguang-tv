"""Render the tracked SVG in CI; generated binaries stay in dist/icons."""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SIZES = (16, 24, 32, 48, 64, 128, 256)


def generate():
    import cairosvg
    from PIL import Image

    output = ROOT / "dist/icons"
    output.mkdir(parents=True, exist_ok=True)
    svg = (ROOT / "assets/icon.svg").read_bytes()
    # Render every size independently so small icons are not downsampled twice.
    for size in (*SIZES, 512, 1024):
        cairosvg.svg2png(bytestring=svg, output_width=size, output_height=size,
                        write_to=str(output / f"liuguangtv-{size}.png"))
    with Image.open(output / "liuguangtv-256.png") as image:
        frames = []
        for size in SIZES[:-1]:
            with Image.open(output / f"liuguangtv-{size}.png") as frame:
                frames.append(frame.copy())
        image.save(output / "liuguangtv.ico", format="ICO",
                   sizes=[(size, size) for size in SIZES], append_images=frames)
    # Fail CI rather than ship an incomplete or unreadable ICO.
    with Image.open(output / "liuguangtv.ico") as icon:
        assert icon.ico.sizes() == {(size, size) for size in SIZES}
        for size in SIZES:
            assert icon.ico.getimage((size, size)).size == (size, size)
    print(f"Generated and verified icons in {output}")


if __name__ == "__main__":
    generate()
