#pragma once

#if defined (__cplusplus)
extern "C" {
#endif

int LZ4_compressHC2 (const char* source, char* dest, int inputSize, int compressionLevel);

#if defined (__cplusplus)
}
#endif
