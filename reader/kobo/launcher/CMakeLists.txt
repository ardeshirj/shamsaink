# ShamsaInk's rows in Nickel's More tab, a Qt 6 image plugin for Kobo firmware 5.x/6.x, built in kobuild's qt6 image
# (build.sh, make kobo-launcher). The way into Nickel follows NickelModManager's (MIT), see plugin.cpp.
cmake_minimum_required(VERSION 3.16)
project(shamsaink_launcher LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
find_package(Qt6 REQUIRED COMPONENTS Core Gui Widgets)
add_library(shamsaink-launcher MODULE plugin.cpp)
target_link_libraries(shamsaink-launcher PRIVATE Qt6::Widgets Qt6::Gui ${CMAKE_DL_LIBS})
target_compile_options(shamsaink-launcher PRIVATE -Wall -Wextra)
set_target_properties(shamsaink-launcher PROPERTIES CXX_VISIBILITY_PRESET hidden)
