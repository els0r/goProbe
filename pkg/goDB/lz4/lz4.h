#pragma once

#if defined (__cplusplus)
extern "C" {
#endif


/**************************************
   Version
**************************************/
#define LZ4_VERSION_MAJOR    1    /* for major interface/format changes  */
#define LZ4_VERSION_MINOR    3    /* for minor interface/format changes  */
#define LZ4_VERSION_RELEASE  0    /* for tweaks, bug-fixes, or development */
#define LZ4_VERSION_NUMBER (LZ4_VERSION_MAJOR *100*100 + LZ4_VERSION_MINOR *100 + LZ4_VERSION_RELEASE)
int LZ4_versionNumber (void);

/**************************************
   Tuning parameter
**************************************/
/*
 * LZ4_MEMORY_USAGE :
 * Memory usage formula : N->2^N Bytes (examples : 10 -> 1KB; 12 -> 4KB ; 16 -> 64KB; 20 -> 1MB; etc.)
 * Increasing memory usage improves compression ratio
 * Reduced memory usage can improve speed, due to cache effect
 * Default value is 14, for 16KB, which nicely fits into Intel x86 L1 cache
 */
#define LZ4_MEMORY_USAGE 14

/**************************************
   Advanced Functions
**************************************/
#define LZ4_MAX_INPUT_SIZE        0x7E000000   /* 2 113 929 216 bytes */
#define LZ4_COMPRESSBOUND(isize)  ((unsigned int)(isize) > (unsigned int)LZ4_MAX_INPUT_SIZE ? 0 : (isize) + ((isize)/255) + 16)

int LZ4_compressBound(int isize);

int LZ4_decompress_fast (const char* source, char* dest, int originalSize);

#define LZ4_STREAMSIZE_U32 ((1 << (LZ4_MEMORY_USAGE-2)) + 8)
#define LZ4_STREAMSIZE     (LZ4_STREAMSIZE_U32 * sizeof(unsigned int))

#if defined (__cplusplus)
}
#endif
