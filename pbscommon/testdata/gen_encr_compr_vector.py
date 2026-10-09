#!/usr/bin/env python3
"""Generate the ENCR_COMPR golden vector used by crypt_compress_test.go.

Builds a compressed + encrypted PBS DataBlob the way proxmox-backup's
DataBlob::encode(compress = true) does (pbs-datastore/src/data_blob.rs):
zstd::bulk::compress(data, 1) -- a regular zstd frame from libzstd, the
library the Rust zstd crate binds -- then AES-256-GCM through OpenSSL's
libcrypto, framed as MAGIC || CRC32(ciphertext) || IV || TAG || ciphertext.
Only the Python standard library, libzstd and libcrypto are needed:

    python3 pbscommon/testdata/gen_encr_compr_vector.py
"""
import ctypes
import ctypes.util
import zlib

zstd = ctypes.CDLL(ctypes.util.find_library("zstd"))
zstd.ZSTD_compressBound.restype = ctypes.c_size_t
zstd.ZSTD_compressBound.argtypes = [ctypes.c_size_t]
zstd.ZSTD_compress.restype = ctypes.c_size_t
zstd.ZSTD_compress.argtypes = [ctypes.c_char_p, ctypes.c_size_t, ctypes.c_char_p, ctypes.c_size_t, ctypes.c_int]
zstd.ZSTD_isError.argtypes = [ctypes.c_size_t]

lib = ctypes.CDLL(ctypes.util.find_library("crypto"))
lib.EVP_CIPHER_CTX_new.restype = ctypes.c_void_p
lib.EVP_aes_256_gcm.restype = ctypes.c_void_p
lib.EVP_EncryptInit_ex.argtypes = [ctypes.c_void_p] * 5
lib.EVP_EncryptUpdate.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.POINTER(ctypes.c_int), ctypes.c_char_p, ctypes.c_int]
lib.EVP_EncryptFinal_ex.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.POINTER(ctypes.c_int)]
lib.EVP_CIPHER_CTX_ctrl.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_void_p]
lib.EVP_CIPHER_CTX_free.argtypes = [ctypes.c_void_p]
EVP_CTRL_GCM_SET_IVLEN, EVP_CTRL_GCM_GET_TAG = 0x9, 0x10


def zstd_compress(data, level):
    out = ctypes.create_string_buffer(zstd.ZSTD_compressBound(len(data)))
    n = zstd.ZSTD_compress(out, len(out), data, len(data), level)
    assert not zstd.ZSTD_isError(n)
    return out.raw[:n]


def aes_256_gcm_encrypt(key, iv, data):
    ctx = lib.EVP_CIPHER_CTX_new()
    assert lib.EVP_EncryptInit_ex(ctx, lib.EVP_aes_256_gcm(), None, None, None) == 1
    assert lib.EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, len(iv), None) == 1
    assert lib.EVP_EncryptInit_ex(ctx, None, None, key, iv) == 1
    out = ctypes.create_string_buffer(len(data) + 16)
    n = ctypes.c_int(0)
    assert lib.EVP_EncryptUpdate(ctx, out, ctypes.byref(n), data, len(data)) == 1
    total = n.value
    fin = ctypes.create_string_buffer(16)
    assert lib.EVP_EncryptFinal_ex(ctx, fin, ctypes.byref(n)) == 1
    tag = ctypes.create_string_buffer(16)
    assert lib.EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_GET_TAG, 16, tag) == 1
    lib.EVP_CIPHER_CTX_free(ctx)
    return out.raw[:total], tag.raw


ENCR_COMPR_BLOB_MAGIC = bytes([230, 89, 27, 191, 11, 191, 216, 11])
key = bytes(range(32))
iv = bytes(range(0x64, 0x74))
plain = b"Proxmox Backup compressed encrypted blob test vector. " * 8

compressed = zstd_compress(plain, 1)
assert len(compressed) < len(plain)
ct, tag = aes_256_gcm_encrypt(key, iv, compressed)
blob = ENCR_COMPR_BLOB_MAGIC + zlib.crc32(ct).to_bytes(4, "little") + iv + tag + ct
print("plain:", plain[:54].decode(), "x 8")
print("blob:", blob.hex())
