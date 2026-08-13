#include "sobel.h"

#include <cassert>
#include <cstdint>
#include <vector>

int main() {
    const std::vector<std::uint8_t> input{
        0, 0, 255, 255, 255,
        0, 0, 255, 255, 255,
        0, 0, 255, 255, 255,
        0, 0, 255, 255, 255,
        0, 0, 255, 255, 255,
    };
    const auto output = sobel_cpu(input, 5, 5);
    for (int x = 0; x < 5; ++x) {
        assert(output[x] == 0);
        assert(output[20 + x] == 0);
    }
    for (int y = 0; y < 5; ++y) {
        assert(output[y * 5] == 0);
        assert(output[y * 5 + 4] == 0);
    }
    assert(output[6] == 255);
    assert(output[7] == 255);
    assert(output[8] == 0);
    return 0;
}

