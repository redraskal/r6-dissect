#pragma endian little
#pragma pattern_limit 500000

#include <std/sys.pat>

struct HeaderProperty {
    u8 keyLen [[name("Key Length")]];
    padding[7];
    char key[keyLen] [[name("Key")]];
    u8 valueLen [[name("Value Length")]];
    padding[7];
    char value[valueLen] [[name("Value")]];
};

struct Header {
    char magic[7] [[name("Magic"), comment("Decompressed Dissect files always start with this sequence.")]];
    std::assert(magic == "dissect", "Incorrect magic");

    padding[28];

    u8 size [[name("Properties Size"), comment("The number of header properties.")]];
    padding[7];
    HeaderProperty properties[size] [[name("Properties")]];
};

Header header @0x00;

// unknown, contains incrementing numbers
struct InitTableItem {
    u24 unknown;
    u8 unknown2;
    u8 unknown3;
    u8 unknown4;
    u8 unknown5;
    padding[3];
    u8 unknown6;
    u8 unknown7;
};

struct InitTable {
    padding[8];
    u16 size;
    padding[21];
    InitTableItem items[size];
};

InitTable initTable @sizeof(header);
