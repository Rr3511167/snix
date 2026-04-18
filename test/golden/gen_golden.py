"""
Generate golden test vectors from the Python upstream ClientHelloMaker.
Run this from the upstream SNI-Spoofing/utils dir on PYTHONPATH.
Produces golden.json for byte-for-byte verification in Go tests.
"""
import json
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "SNI-Spoofing"))
from utils.packet_templates import ClientHelloMaker

def vec(name, rnd_hex, sess_hex, sni, ks_hex):
    rnd = bytes.fromhex(rnd_hex)
    sess = bytes.fromhex(sess_hex)
    ks = bytes.fromhex(ks_hex)
    out = ClientHelloMaker.get_client_hello_with(rnd, sess, sni.encode(), ks)
    return {
        "name": name,
        "rnd": rnd_hex,
        "sess_id": sess_hex,
        "sni": sni,
        "key_share": ks_hex,
        "expected_hex": out.hex(),
        "expected_len": len(out),
    }

vectors = [
    vec("all_zero_auth_vercel", "00"*32, "00"*32, "auth.vercel.com", "00"*32),
    vec("all_zero_mci_ir",      "00"*32, "00"*32, "mci.ir",          "00"*32),
    vec("ff_pattern_short_sni", "ff"*32, "aa"*32, "x.io",            "55"*32),
    vec("mixed_long_sni",       "deadbeef"*8, "cafebabe"*8, "very-long-subdomain.example.co.uk", "13371337"*8),
    vec("single_char_sni",      "01"*32, "02"*32, "a",               "03"*32),
]

golden = {"version": 1, "vectors": vectors}
out_path = os.path.join(os.path.dirname(__file__), "clienthello_golden.json")
with open(out_path, "w") as f:
    json.dump(golden, f, indent=2)
print(f"wrote {len(vectors)} vectors to {out_path}")
for v in vectors:
    print(f"  {v['name']}: {v['expected_len']} bytes")
