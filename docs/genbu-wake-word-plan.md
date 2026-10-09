# Plan: a "Hey Genbu" wake word

The Show's wake words are microWakeWord models. "Hey Genbu" isn't one of them, so it has to be
trained. Before any training, this page sorts out what each part of the training is made from and
what that means for the model. The model is meant to be published with the rest of this project, for
anyone to use, change and pass on, and maybe to ship on Shows one day. So nothing that goes into it
may say "non-commercial": an open licence can't take that freedom back from the people downstream.
It isn't legal advice: check the licence file in each download before using it, as the notes below
say where a source was unclear.

## What can be passed on today

The models the Show already has (Okay Nabu, Hey Jarvis and the rest) come from
[esphome/micro-wake-word-models](https://github.com/esphome/micro-wake-word-models), under Apache
2.0. The repository says that covers the trained weights and their manifests too, with no extra
terms. So publishing and shipping them is fine, with the licence and NOTICE kept. The same repository warns
that a wake word's name can be someone else's mark. "Jarvis" is a Marvel character, which matters
more for what a product is called than for the model.

## What training uses, and its licence

| Part | Source | Licence | Fine for an open model? |
| --- | --- | --- | --- |
| Training code | [micro-wake-word](https://github.com/OHF-Voice/micro-wake-word) | Apache 2.0 | Yes |
| Spoken "Hey Genbu" samples | [piper-sample-generator](https://github.com/rhasspy/piper-sample-generator), a voice trained on LibriTTS-R | code MIT; LibriTTS-R CC BY 4.0 on [OpenSLR](https://www.openslr.org/141/) | Yes, with attribution (one catalogue lists LibriTTS-R as CC BY-NC-ND, so check the download) |
| Ready-made negatives (everything that isn't the wake word) | [kahrendt/microwakeword](https://huggingface.co/datasets/kahrendt/microwakeword): FMA, FSD50K, WHAM, LibriSpeech, VOiCES, CHiME-6, DiPCo | CC BY-NC 4.0 as a whole | **No.** Non-commercial |

The negatives are the problem. The ready-made set is what the microWakeWord notebooks download, and
it is licensed non-commercial. A model trained on it inherits that doubt, and whoever builds a
device with it would inherit it too. It's fine for a first test at home that nobody else gets.
The model we publish is trained on our own negatives, from sources that allow it.

## Negatives we can use

| Source | Licence | Note |
| --- | --- | --- |
| [MUSAN](https://openslr.org/17/) (music, speech, noise) | CC BY 4.0 on OpenSLR; per-file public domain or CC, and its authors say they only took content that allows commercial use | Keep the per-file attribution |
| LibriSpeech | CC BY 4.0 | English speech |
| Common Voice, German | CC0 | German speech, which a German household needs most |
| Recordings from our own rooms | ours | TV, German music, talk at the table. The best negatives there are, and exactly what the Show gets wrong today |

Left out unless their licence checks out: FMA and FSD50K (per-track or per-clip licences, some
non-commercial; usable only filtered to CC0 and CC BY), WHAM! (licence not found), CHiME-6
(OpenSLR now lists [CC BY-SA 4.0](https://openslr.org/150), but it was first handed out under
[non-commercial and commercial licences](https://auditory.org/postings/2019/686.html)), and any room
impulse response set until we have picked one with a licence that allows it.

## The phrase

"Hey Genbu" rather than "Genbu" alone: three syllables are much harder to hear by accident than two,
and "Okay Nabu" and "Hey Jarvis" are that length for the same reason.

## Steps

1. Generate a few thousand "Hey Genbu" samples with piper-sample-generator, plus near-misses
   ("Hey Gen", "Genre", "Hey Benjamin") as negatives.
2. Record real "Hey Genbu"s from the people who will use it, with their OK. A few dozen each makes
   a clear difference over synthetic voices alone.
3. Build the negative features from the sources above, and from a few hours of our own rooms.
4. Train with micro-wake-word, then test on the Show itself: false wakes per hour with the TV and
   German music on, and how many real tries it catches, from the `wake detected` and
   `wake near miss` lines in its log.
5. Ship it next to Okay Nabu with a list of the CC BY sources it was trained from.
