from collections import deque
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "assets" / "classpoints-icon-source.png"
PNG_OUTPUT = ROOT / "assets" / "classpoints-icon.png"
ICO_OUTPUT = ROOT / "assets" / "classpoints.ico"


def is_background(pixel: tuple[int, int, int]) -> bool:
    red, green, blue = pixel
    return min(pixel) >= 200 and max(pixel) - min(pixel) <= 20


def remove_generated_checkerboard(image: Image.Image) -> Image.Image:
    rgb = image.convert("RGB")
    width, height = rgb.size
    pixels = rgb.load()
    visited = bytearray(width * height)
    queue: deque[tuple[int, int]] = deque()

    def enqueue(x: int, y: int) -> None:
        index = y * width + x
        if not visited[index] and is_background(pixels[x, y]):
            visited[index] = 1
            queue.append((x, y))

    for x in range(width):
        enqueue(x, 0)
        enqueue(x, height - 1)
    for y in range(height):
        enqueue(0, y)
        enqueue(width - 1, y)

    while queue:
        x, y = queue.popleft()
        if x > 0:
            enqueue(x - 1, y)
        if x + 1 < width:
            enqueue(x + 1, y)
        if y > 0:
            enqueue(x, y - 1)
        if y + 1 < height:
            enqueue(x, y + 1)

    rgba = rgb.convert("RGBA")
    alpha = Image.new("L", rgb.size, 255)
    alpha_pixels = alpha.load()
    for y in range(height):
        offset = y * width
        for x in range(width):
            if visited[offset + x]:
                alpha_pixels[x, y] = 0
    rgba.putalpha(alpha)
    return rgba


def fit_square(image: Image.Image, size: int = 1024, padding: int = 54) -> Image.Image:
    alpha = image.getchannel("A")
    bounds = alpha.getbbox()
    if not bounds:
        raise RuntimeError("图标主体为空")
    subject = image.crop(bounds)
    available = size - padding * 2
    subject.thumbnail((available, available), Image.Resampling.LANCZOS)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    position = ((size - subject.width) // 2, (size - subject.height) // 2)
    canvas.alpha_composite(subject, position)
    return canvas


def main() -> None:
    icon = fit_square(remove_generated_checkerboard(Image.open(SOURCE)))
    icon.save(PNG_OUTPUT, optimize=True)
    icon.save(
        ICO_OUTPUT,
        format="ICO",
        sizes=[(16, 16), (20, 20), (24, 24), (32, 32), (40, 40), (48, 48), (64, 64), (128, 128), (256, 256)],
    )
    print(f"已生成：{PNG_OUTPUT}")
    print(f"已生成：{ICO_OUTPUT}")


if __name__ == "__main__":
    main()
