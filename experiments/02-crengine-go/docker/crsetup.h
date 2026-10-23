#ifndef CRSETUP_H_INCLUDED
#define CRSETUP_H_INCLUDED

/// KOReader's crsetup.h.cmake (koreader-base 6e4bc81), with the formats and image types this reader does not need
/// switched off so it builds on Debian's libraries alone.
/// Yes, even on macOS (Windows is unsupported)…
#define LINUX 1
#define _LINUX 1

/// Compression.
#define USE_ZLIB                             1
#define USE_ZSTD                             1

/// Debugging.

/// Documents cache.
#define DOCUMENT_CACHING_MIN_SIZE            0x10000   //  64.0 KiB
#define DOCUMENT_CACHING_SIZE_THRESHOLD      0x100000  //   1.0 MiB

/// Document formats.
#define CHM_SUPPORT_ENABLED                  0
#define CR3_ANTIWORD_PATCH                   0
#define ENABLE_ANTIWORD                      0
#define USE_MD4C                             0

/// I18N
#define CR_EMULATE_GETTEXT                   1

/// Images.
#define ARBITRARY_IMAGE_SCALE_ENABLED        1
#define MAX_IMAGE_SCALE_MUL                  2
#define USE_GIF                              0
#define USE_LIBJPEG                          1
#define USE_LIBPNG                           1
#define USE_LIBWEBP                          0
#define USE_LUNASVG                          0
#define USE_NANOSVG                          0  // better SVG support via LunaSVG

/// Miscellaneous.
#define MATHML_SUPPORT                       1
#define USE_SRELL_REGEX                      0

/// Output buffer.
#define COLOR_BACKBUFFER                     1
#define CR_INTERNAL_PAGE_ORIENTATION         1
#define GRAY_BACKBUFFER_BITS                 2
#define GRAY_INVERSE                         0

/// Streams.
#define HAVE_OFF64_T                         1
#ifdef HAVE_OFF64_T
# define HAVE_STAT64                         1
# define _LARGEFILE64_SOURCE                 1
#endif
#define LVLONG_FILE_SUPPORT                  0
#define USE_ANSI_FILES                       1
#define FILE_STREAM_BUFFER_SIZE              0x20000   // 128.0 KiB
#define ZIP_STREAM_BUFFER_SIZE               0x40000   // 256.0 KiB

/// System.
#define CR_USE_THREADS                       0
#define LDOM_USE_OWN_MEM_MAN                 1

/// Text.
#define USE_LIMITED_FONT_SIZES_SET           0
#define USE_BITMAP_FONTS                     0
#define USE_WIN32_FONTS                      0
#define USE_GLYPHCACHE_HASHTABLE             0
#define GLYPH_CACHE_SIZE                     0x40000   // 256.0 KiB
#define ALLOW_KERNING                        1
#define USE_FONTCONFIG                       0
#define USE_FREETYPE                         1
#define USE_FRIBIDI                          1
#define USE_HARFBUZZ                         1
#define USE_LIBUNIBREAK                      1
#define USE_UTF8PROC                         1

/// Disable unused code.
#define CR_ENABLE_PAGE_IMAGE_CACHE           0

#endif//CRSETUP_H_INCLUDED

// vim: ft=cpp
