#include "sobel.h"

#define STB_IMAGE_IMPLEMENTATION
#define STBI_ONLY_JPEG
#define STBI_ONLY_PNG
#include "stb_image.h"
#define STB_IMAGE_WRITE_IMPLEMENTATION
#include "stb_image_write.h"

#include <chrono>
#include <filesystem>
#include <iomanip>
#include <iostream>
#include <stdexcept>
#include <string>

namespace {
struct Args { std::string input; std::string output; std::string device{"auto"}; };

Args parse_args(int argc, char** argv) {
    Args args;
    for (int i = 1; i < argc; ++i) {
        const std::string key = argv[i];
        if ((key == "--input" || key == "--output" || key == "--device") && i + 1 < argc) {
            const std::string value = argv[++i];
            if (key == "--input") args.input = value;
            else if (key == "--output") args.output = value;
            else args.device = value;
        } else {
            throw std::invalid_argument("usage: cudaops-process --input FILE --output FILE --device auto|cpu|cuda");
        }
    }
    if (args.input.empty() || args.output.empty() ||
        (args.device != "auto" && args.device != "cpu" && args.device != "cuda")) {
        throw std::invalid_argument("usage: cudaops-process --input FILE --output FILE --device auto|cpu|cuda");
    }
    return args;
}
}

int main(int argc, char** argv) {
    const auto started = std::chrono::steady_clock::now();
    try {
        const auto args = parse_args(argc, argv);
        int width = 0, height = 0, source_channels = 0;
        stbi_uc* pixels = stbi_load(args.input.c_str(), &width, &height, &source_channels, 3);
        if (!pixels) throw std::runtime_error(std::string("decode failed: ") + stbi_failure_reason());
        auto gray = grayscale(pixels, width, height, 3);
        stbi_image_free(pixels);

        std::vector<std::uint8_t> output;
        std::string used = "cpu";
        bool fallback = false;
        double kernel_ms = 0;
        if (args.device != "cpu") {
            const auto result = sobel_cuda(gray, output, width, height);
            if (result.available && result.error.empty()) {
                used = "cuda";
                kernel_ms = result.kernel_ms;
            } else if (args.device == "cuda" || result.available) {
                throw std::runtime_error("CUDA processing failed: " + result.error);
            } else {
                fallback = true;
                output = sobel_cpu(gray, width, height);
            }
        } else {
            output = sobel_cpu(gray, width, height);
        }

        const auto output_parent = std::filesystem::path(args.output).parent_path();
        if (!output_parent.empty()) std::filesystem::create_directories(output_parent);
        if (!stbi_write_png(args.output.c_str(), width, height, 1, output.data(), width)) {
            throw std::runtime_error("failed to write PNG output");
        }
        const auto total_ms = std::chrono::duration<double, std::milli>(
            std::chrono::steady_clock::now() - started).count();
        std::cout << std::fixed << std::setprecision(3)
                  << "{\"device\":\"" << used << "\",\"fallback_used\":" << (fallback ? "true" : "false")
                  << ",\"width\":" << width << ",\"height\":" << height
                  << ",\"total_ms\":" << total_ms << ",\"kernel_ms\":" << kernel_ms << "}\n";
        return 0;
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
