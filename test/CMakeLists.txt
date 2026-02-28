cmake_minimum_required(VERSION 3.22.1)

project("test")

include_directories(${CMAKE_CURRENT_SOURCE_DIR}
        ${CMAKE_CURRENT_SOURCE_DIR}/../)
add_library(testlib lib.c)
add_executable(test main.c)
set_target_properties(test PROPERTIES ENABLE_EXPORTS TRUE)
target_link_libraries(test testlib dl)