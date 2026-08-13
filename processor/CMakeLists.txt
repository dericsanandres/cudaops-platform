cmake_minimum_required(VERSION 3.28)
project(cudaops_processor LANGUAGES CXX)

option(CUDAOPS_ENABLE_CUDA "Build the CUDA implementation" ON)
option(BUILD_TESTING "Build processor tests" OFF)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

add_library(cudaops_sobel src/sobel.cpp)
target_include_directories(cudaops_sobel PUBLIC src)
target_compile_options(cudaops_sobel PRIVATE -Wall -Wextra -Wpedantic)

if(CUDAOPS_ENABLE_CUDA)
  enable_language(CUDA)
  add_library(cudaops_cuda src/sobel_cuda.cu)
  target_include_directories(cudaops_cuda PUBLIC src)
  target_compile_definitions(cudaops_cuda PUBLIC CUDAOPS_HAS_CUDA=1)
  target_compile_features(cudaops_cuda PUBLIC cxx_std_20)
  set_target_properties(cudaops_cuda PROPERTIES CUDA_ARCHITECTURES 120)
else()
  add_library(cudaops_cuda src/sobel_cuda_stub.cpp)
  target_include_directories(cudaops_cuda PUBLIC src)
endif()

add_executable(cudaops-process src/main.cpp)
target_include_directories(cudaops-process PRIVATE third_party)
target_link_libraries(cudaops-process PRIVATE cudaops_sobel cudaops_cuda)
target_compile_options(cudaops-process PRIVATE -Wall -Wextra -Wpedantic)

include(CTest)
if(BUILD_TESTING)
  add_executable(sobel_test tests/sobel_test.cpp)
  target_link_libraries(sobel_test PRIVATE cudaops_sobel)
  add_test(NAME sobel_unit COMMAND sobel_test)
endif()

install(TARGETS cudaops-process RUNTIME DESTINATION bin)

