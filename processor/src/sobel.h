#pragma once

#include <cstdint>
#include <string>
#include <vector>

struct CudaResult {
    bool available{false};
    double kernel_ms{0};
    std::string error;
};

std::vector<std::uint8_t> grayscale(const std::uint8_t* rgb, int width, int height, int channels);
std::vector<std::uint8_t> sobel_cpu(const std::vector<std::uint8_t>& gray, int width, int height);
CudaResult sobel_cuda(const std::vector<std::uint8_t>& gray, std::vector<std::uint8_t>& output,
                      int width, int height);

