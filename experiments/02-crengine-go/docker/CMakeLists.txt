# crengine as one static library, built the way koreader-base's thirdparty/kpvcrlib/CMakeLists.txt does it (same
# sources and flags), against Debian's FreeType, HarfBuzz, FriBidi, libunibreak, utf8proc, zstd, PNG, JPEG and zlib.
cmake_minimum_required(VERSION 3.17)
project(crengine LANGUAGES C CXX)
find_package(PkgConfig REQUIRED)
pkg_check_modules(DEPS REQUIRED freetype2 harfbuzz fribidi libunibreak libutf8proc libzstd libpng libjpeg zlib libxxhash)

set(CRE_DIR ${CMAKE_CURRENT_SOURCE_DIR}/crengine/crengine)
add_library(crengine STATIC
    ${CRE_DIR}/qimagescale/qimagescale.cpp
    ${CRE_DIR}/src/chmfmt.cpp
    ${CRE_DIR}/src/cp_stats.cpp
    ${CRE_DIR}/src/cri18n.cpp
    ${CRE_DIR}/src/crtxtenc.cpp
    ${CRE_DIR}/src/docxfmt.cpp
    ${CRE_DIR}/src/epubfmt.cpp
    ${CRE_DIR}/src/fb3fmt.cpp
    ${CRE_DIR}/src/hist.cpp
    ${CRE_DIR}/src/hyphman.cpp
    ${CRE_DIR}/src/lstridmap.cpp
    ${CRE_DIR}/src/lvdocview.cpp
    ${CRE_DIR}/src/lvdrawbuf.cpp
    ${CRE_DIR}/src/lvfnt.cpp
    ${CRE_DIR}/src/lvfntman.cpp
    ${CRE_DIR}/src/lvimg.cpp
    ${CRE_DIR}/src/lvmemman.cpp
    ${CRE_DIR}/src/lvopc.cpp
    ${CRE_DIR}/src/lvpagesplitter.cpp
    ${CRE_DIR}/src/lvrend.cpp
    ${CRE_DIR}/src/lvstream.cpp
    ${CRE_DIR}/src/lvstring.cpp
    ${CRE_DIR}/src/lvstsheet.cpp
    ${CRE_DIR}/src/lvstyles.cpp
    ${CRE_DIR}/src/lvtextfm.cpp
    ${CRE_DIR}/src/lvtinydom.cpp
    ${CRE_DIR}/src/lvxml.cpp
    ${CRE_DIR}/src/mathml.cpp
    ${CRE_DIR}/src/mdfmt.cpp
    ${CRE_DIR}/src/odtfmt.cpp
    ${CRE_DIR}/src/odxutil.cpp
    ${CRE_DIR}/src/pdbfmt.cpp
    ${CRE_DIR}/src/props.cpp
    ${CRE_DIR}/src/renderutil.cpp
    ${CRE_DIR}/src/rtfimp.cpp
    ${CRE_DIR}/src/textlang.cpp
    ${CRE_DIR}/src/txtselector.cpp
    ${CRE_DIR}/src/wordfmt.cpp
)
target_compile_features(crengine PRIVATE cxx_std_17)
target_compile_options(crengine PRIVATE -ftabstop=4 -Wall -Wno-unused -include ${CMAKE_CURRENT_SOURCE_DIR}/crsetup.h)
target_include_directories(crengine PUBLIC ${CRE_DIR}/include ${DEPS_INCLUDE_DIRS})
