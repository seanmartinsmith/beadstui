# /// script
# requires-python = ">=3.13"
# dependencies = ["pillow"]
# ///
"""Convert images in current directory to WebP format."""

import os
from concurrent.futures import ProcessPoolExecutor, as_completed
from pathlib import Path


def convert_image_to_webp(paths: tuple[Path, Path]) -> str:
    """Convert a single image to WebP format."""
    from PIL import Image
    
    input_path, output_path = paths
    with Image.open(input_path) as img:
        img.save(output_path, "WEBP", lossless=False, quality=55, method=6)
    return input_path.name


def batch_convert_to_webp(folder: Path) -> None:
    """Batch convert all PNG/JPG images in folder to WebP."""
    extensions = {".png", ".jpg", ".jpeg"}
    image_files = [
        f for f in folder.iterdir()
        if f.is_file() and f.suffix.lower() in extensions
    ]

    if not image_files:
        print("No images found to convert.")
        return

    tasks = [
        (img, img.with_suffix(".webp"))
        for img in image_files
    ]

    workers = min(os.cpu_count() or 4, len(tasks))
    print(f"Converting {len(tasks)} images using {workers} workers...")

    with ProcessPoolExecutor(max_workers=workers) as executor:
        futures = {executor.submit(convert_image_to_webp, t): t for t in tasks}
        for i, future in enumerate(as_completed(futures), 1):
            name = future.result()
            print(f"[{i}/{len(tasks)}] Converted: {name}")

    print("Done!")


if __name__ == "__main__":
    batch_convert_to_webp(Path.cwd())