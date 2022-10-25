# SPDX-License-Identifier: MIT
# Copyright (c) 2018 QRI, Inc.
# Copyright (c) 2022 Canonical Ltd.

load("assert.star", "assert")
load("re.star", "re")

def test_find():
    result = re.find(
        r"(\w*)\s*(ADD|REM|DEL|EXT|TRF)\s*(.*)\s*(NAT|INT)\s*(.*)\s*(\(\w{2}\))\s*(.*)",
        "EDM ADD FROM INJURED NAT Jordan BEAULIEU (DB) Western University"
    )
    assert.eq(result.groups(), (
        "EDM ADD FROM INJURED NAT Jordan BEAULIEU (DB) Western University",
        "EDM",
        "ADD",
        "FROM INJURED ",
        "NAT",
        "Jordan BEAULIEU ",
        "(DB)",
        "Western University",
    ))

def test_findall():
    result = re.findall(
        r"\b([A-Z])\w+",
        "Alpha Bravo charlie DELTA Echo F Hotel",
    )
    result_groups = [m.groups() for m in result]
    assert.eq(result_groups, [
        ("Alpha", "A"),
        ("Bravo", "B"),
        ("DELTA", "D"),
        ("Echo", "E"),
        ("Hotel", "H"),
    ])

def test_split():
    result = re.split(r"\s+", "  foo bar   baz\tqux quux ", 5)
    assert.eq(result, ["", "foo", "bar", "baz", "qux quux "])

def test_sub():
    result = re.sub(r"[A-Z]+", "_", "aBcDEFefG")
    assert.eq(result, "a_c_ef_")
