#include "sobel.h"

#include <cuda_runtime.h>

#include <string>

namespace {
__global__ void sobel_kernel(const std::uint8_t* input, std::uint8_t* output, int width, int height) {
    const int x = blockIdx.x * blockDim.x + threadIdx.x;
    const int y = blockIdx.y * blockDim.y + threadIdx.y;
    if (x >= width || y >= height) return;
    const int index = y * width + x;
    if (x == 0 || y == 0 || x == width - 1 || y == height - 1) {
        output[index] = 0;
        return;
    }
    const int top_left = input[(y - 1) * width + x - 1];
    const int top = input[(y - 1) * width + x];
    const int top_right = input[(y - 1) * width + x + 1];
    const int left = input[y * width + x - 1];
    const int right = input[y * width + x + 1];
    const int bottom_left = input[(y + 1) * width + x - 1];
    const int bottom = input[(y + 1) * width + x];
    const int bottom_right = input[(y + 1) * width + x + 1];
    const int gx = -top_left + top_right - 2 * left + 2 * right - bottom_left + bottom_right;
    const int gy = -top_left - 2 * top - top_right + bottom_left + 2 * bottom + bottom_right;
    const int magnitude = abs(gx) + abs(gy);
    output[index] = static_cast<std::uint8_t>(magnitude > 255 ? 255 : magnitude);
}

std::string cuda_error(cudaError_t status) { return cudaGetErrorString(status); }
}

CudaResult sobel_cuda(const std::vector<std::uint8_t>& gray, std::vector<std::uint8_t>& output,
                      int width, int height) {
    int count = 0;
    auto status = cudaGetDeviceCount(&count);
    if (status != cudaSuccess || count == 0) {
        return {.available = false, .kernel_ms = 0,
                .error = status == cudaSuccess ? "no CUDA device available" : cuda_error(status)};
    }
    cudaDeviceProp properties{};
    if ((status = cudaGetDeviceProperties(&properties, 0)) != cudaSuccess || properties.major < 12) {
        return {.available = false, .kernel_ms = 0,
                .error = status == cudaSuccess ? "CUDA device is not compatible with sm_120" : cuda_error(status)};
    }

    const std::size_t bytes = gray.size();
    std::uint8_t* device_input = nullptr;
    std::uint8_t* device_output = nullptr;
    cudaEvent_t start = nullptr;
    cudaEvent_t stop = nullptr;
    auto cleanup = [&] {
        if (start) cudaEventDestroy(start);
        if (stop) cudaEventDestroy(stop);
        if (device_input) cudaFree(device_input);
        if (device_output) cudaFree(device_output);
    };

    if ((status = cudaMalloc(&device_input, bytes)) != cudaSuccess ||
        (status = cudaMalloc(&device_output, bytes)) != cudaSuccess ||
        (status = cudaMemcpy(device_input, gray.data(), bytes, cudaMemcpyHostToDevice)) != cudaSuccess ||
        (status = cudaEventCreate(&start)) != cudaSuccess ||
        (status = cudaEventCreate(&stop)) != cudaSuccess) {
        const auto error = cuda_error(status);
        cleanup();
        return {.available = true, .kernel_ms = 0, .error = error};
    }

    const dim3 block(16, 16);
    const dim3 grid((width + block.x - 1) / block.x, (height + block.y - 1) / block.y);
    cudaEventRecord(start);
    sobel_kernel<<<grid, block>>>(device_input, device_output, width, height);
    cudaEventRecord(stop);
    status = cudaEventSynchronize(stop);
    if (status == cudaSuccess) status = cudaGetLastError();
    float elapsed = 0;
    if (status == cudaSuccess) status = cudaEventElapsedTime(&elapsed, start, stop);
    output.resize(bytes);
    if (status == cudaSuccess) {
        status = cudaMemcpy(output.data(), device_output, bytes, cudaMemcpyDeviceToHost);
    }
    const auto error = status == cudaSuccess ? std::string{} : cuda_error(status);
    cleanup();
    return {.available = true, .kernel_ms = elapsed, .error = error};
}
