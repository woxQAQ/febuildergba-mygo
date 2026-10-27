package game

import (
	"crypto/sha256"
	"fmt"
)

// These slot assignments and bitmap fingerprints come from the pinned FE8U
// NarrowFont MenuLowercase and MenuUppercase installers. No font assets or
// legacy runtime are required. Each alias is enabled only for a matching glyph
// reached through the ROM's menu font table, including relocated glyphs.
var narrowMenuGlyphs = map[byte]struct{ text, digest string }{
	0x81: {"a", "ed72a31c427040948a903a96669eff9136e8abc2717e8e229862b98fdce792d2"},
	0x82: {"b", "4e4146f5ba61092495ee404771cc6ebb2b4702f86151aad644e7a523ed006f6b"},
	0x83: {"c", "b066e266a5af419150a8b7fb73ac7fe8fb272425ea879f13c070b2293ec7d941"},
	0x84: {"d", "14302b262c31fb4fca28b684890a71002b1abe122b358c9ea4832ce9fe42a39c"},
	0x85: {"e", "514bea5d404d82553e731d732fcdcbb227634556b3ba38a022b7eaf3c7cedc78"},
	0x86: {"f", "77ec193c13c03c2326e43a03c695c829c3c1ee4f087f5015f12cffaf3e0a1031"},
	0x87: {"g", "e14d9234b78d9989345431c4392ab7858eb3cdb534c7d3f3e63695e4acf83c31"},
	0x88: {"h", "a5c2cb8fff623384db70d62c548d15920db5d525c8fd4059d7584a22b871c0b7"},
	0x89: {"j", "1f5c6f2a08819c97ed2eaeb1c4a86bd862e2a8f98c49e4459033de5182b4ae7a"},
	0x8A: {"k", "a73d203869eb725c67e76835013cec95f4a9849564a2d3f34035fb406cbbdf82"},
	0x8B: {"n", "fa909d84748630466fe6bac13c78e597fa7020da46bbba93b0acc409fc3d3e75"},
	0x8C: {"o", "101c993d69c352042dc3f682d007c02c8c685092a18fd3c59a7e6c6136d22359"},
	0x8D: {"p", "aea809685401d01e8352e50df097f93f7ff19965575c2983753af8a95b249959"},
	0x8E: {"q", "db04bf6046aa84efe59c600ae77d13eabed6d55edb112660722d042991a753e9"},
	0x8F: {"r", "3e8b2f9760784b8134d30ef5f337d787df23e62bbe99fc801d4252d09ebb93a2"},
	0x90: {"s", "620c809aebef311e2cf2da4cc696370447bc3879d6c26c0b5e740d9691b9e5fc"},
	0x95: {"t", "bfce3012e27ad32e87345bfb95bc4149e888090f50a07f8f7b3032a213f1d9a6"},
	0x96: {"u", "eb4abdd282a95ee1bb0b6bde046c2fe3436ac53b3d92bf92a2b84b23755078f0"},
	0x97: {"v", "7772ecb79db9959bb1b81b23d6b8b2c4f2dbc99c45a887f20c375ea178ca6773"},
	0x98: {"x", "7401eafe45b42e2568f5137dbc69d83417c3b68bd893b06f8d2a11880e8489a3"},
	0x99: {"y", "26e462a0871a2e2361334c1c3784444ed7a0577f2608aa8b4279ab95c2c820a9"},
	0x9A: {"z", "0b1208a9cba4481160213cc08cbe55f408c611656cad695e2a44da1eff1f46d6"},
	0x9B: {"A", "7446faa2da94e56cdbb431d7984bc43a663a2557463254a6209915fcaca16b2c"},
	0x9C: {"B", "d473e8346f3f26ebbfd7387ff033c6c268f1ad14e02cbe4229c129b5b38d342e"},
	0x9D: {"C", "f8039b49c7c5d791098e711d2bb18611106b6888f6ca7402abce09bdcc2fedcd"},
	0x9E: {"D", "200c06de4c66bcf634457d8d55225b0a43bdc9e4926cef91915c302e78e8fab8"},
	0x9F: {"E", "022ec526fef557ee307cb79be2a22f2f8bfd0f0347210bcc174da79856d25b87"},
	0xA0: {"F", "b7b4bf648226a7d1fdaf431b8d8ee96e2df14a030c6338e344503820bc5694a0"},
	0xA2: {"G", "9b3c9e419ff7a877bcba57f63a86364c0c34fa79ffee830df03a5c43359e7a73"},
	0xA3: {"H", "30f29feb65f9e92c2999dd71087dff954813c4858722c3a2903a06cd7b7f2716"},
	0xA4: {"J", "742c6dedee9e81cb6acf2831986b25b12b36b6746cc62a0bbe5b4c5a52f4bf23"},
	0xA5: {"K", "2730093fb838009fc46bdf6f6cea4380f496a269ecad41a033df363d8edf5c9d"},
	0xA6: {"L", "c9f210b30f78faea16ed9b81606a4055b62d8911e56fd55498a93a64f057d516"},
	0xA7: {"O", "0ff8f20980c36bb498e9220d15aeb9ea5f5a5b8b64edf7bb45b4ef39e399dec9"},
	0xA8: {"P", "fdaba532445bb95c78cc224228fa397c798ef2d162f21fc1d4fccf610a33cafd"},
	0xA9: {"Q", "9314c528529ac0f15e052df69415d4ae9ebd786cf8520099ffe7122811bdb511"},
	0xAC: {"R", "817b8998e831340602452f0f050dc1f269b92b83607c8e6b1e0704b2f20addd8"},
	0xAD: {"S", "fd4e013da34bbab62cb1f2de68d34e38f8439633a8d6f06f6da458d06a2f312d"},
	0xAE: {"U", "ef5d643d94b630b2dcbffc6afa5898885956c7f3c6a8181428e4153b9c248bf7"},
	0xAF: {"V", "9654cd6b240ce410890626edfb56f9928d0433b1125c7958900413ac178e355e"},
	0xB0: {"X", "bb3600d915ebcece7666f29dbb129b025a4129e4c83b4f512bc6ad7e323747ee"},
	0xB1: {"Y", "af17e55eedba82e00a01d448611ec313862960ba2d101680a4a002e01bfcc9d2"},
	0xB2: {"Z", "15b013c1c1adfd876e5f9b2943a5518cf93ce7df61d7e33b8493102f9b759343"},
	0xBC: {" ", "f5a5fd42d16a20302798ef6ed309979b43003d2320d9f0e8ea9831a92759fb4b"},
}

func fe8uNameAliases(data []byte) map[byte]string {
	aliases := make(map[byte]string)
	for code, signature := range narrowMenuGlyphs {
		at, ok := pointer(data, 0x58C7EC+int(code)*4)
		if !ok || !within(data, at, 72) {
			continue
		}
		// This is the flat, single-byte menu font format, not a UTF-8/SJIS chain.
		if u32(data, at) != 0 || data[at+4] != 0 || data[at+6] != 0 || data[at+7] != 0 || data[at+5] < 1 || data[at+5] > 16 {
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(data[at+8:at+72])) == signature.digest {
			aliases[code] = signature.text
		}
	}
	return aliases
}
