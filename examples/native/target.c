/* SPDX-License-Identifier: EPL-2.0
 * Standalone corpus-replay example: compile as C or C++.
 */
#include <stdio.h>

static int classify(int byte) {
    if (byte == 'A') return 10;
    if (byte == 'B') return 20;
    return 30;
}

int main(int argc, char **argv) {
    if (argc != 2) return 2;
    FILE *input = fopen(argv[1], "rb");
    if (!input) return 3;
    int byte = fgetc(input);
    fclose(input);
    return classify(byte) == 999;
}
