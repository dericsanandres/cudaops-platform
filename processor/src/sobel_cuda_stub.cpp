#include "sobel.h"

CudaResult sobel_cuda(const std::vector<std::uint8_t>&, std::vector<std::uint8_t>&, int, int) {
    return {.available = false, .kernel_ms = 0, .error = "CUDA support was not compiled in"};
}

