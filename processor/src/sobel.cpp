#include "sobel.h"

#include <algorithm>
#include <cstdlib>
#include <stdexcept>

std::vector<std::uint8_t> grayscale(const std::uint8_t* rgb, int width, int height, int channels) {
    if (rgb == nullptr || width <= 0 || height <= 0 || channels < 3) {
        throw std::invalid_argument("invalid RGB image");
    }
    std::vector<std::uint8_t> gray(static_cast<std::size_t>(width) * height);
    for (std::size_t i = 0; i < gray.size(); ++i) {
        const auto base = i * static_cast<std::size_t>(channels);
        // BT.601 coefficients scaled by 256; the shift makes CPU/GPU results exact.
        gray[i] = static_cast<std::uint8_t>((77 * rgb[base] + 150 * rgb[base + 1] +
                                             29 * rgb[base + 2]) >> 8);
    }
    return gray;
}

std::vector<std::uint8_t> sobel_cpu(const std::vector<std::uint8_t>& gray, int width, int height) {
    if (width <= 0 || height <= 0 || gray.size() != static_cast<std::size_t>(width) * height) {
        throw std::invalid_argument("invalid grayscale image");
    }
    std::vector<std::uint8_t> output(gray.size(), 0);
    for (int y = 1; y < height - 1; ++y) {
        for (int x = 1; x < width - 1; ++x) {
            const auto at = [&](int dx, int dy) -> int {
                return gray[static_cast<std::size_t>(y + dy) * width + x + dx];
            };
            const int gx = -at(-1, -1) + at(1, -1) - 2 * at(-1, 0) + 2 * at(1, 0) -
                           at(-1, 1) + at(1, 1);
            const int gy = -at(-1, -1) - 2 * at(0, -1) - at(1, -1) + at(-1, 1) +
                           2 * at(0, 1) + at(1, 1);
            output[static_cast<std::size_t>(y) * width + x] =
                static_cast<std::uint8_t>(std::min(255, std::abs(gx) + std::abs(gy)));
        }
    }
    return output;
}

