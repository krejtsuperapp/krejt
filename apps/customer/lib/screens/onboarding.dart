import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Hyrja e parë: gjashtë foto që tregojnë çfarë bën aplikacioni, me gjuhën lart djathtas.
/// Gjuha ndërrohet në çast (vetëm në pajisje); ruhet kur përdoruesi shtyp "Fillo" (§2).
class OnboardingScreen extends StatefulWidget {
  const OnboardingScreen({super.key});

  static const slides = <OnboardingSlide>[
    OnboardingSlide('assets/onboarding/01.jpg', 'onboarding.s1'),
    OnboardingSlide('assets/onboarding/02.jpg', 'onboarding.s2'),
    OnboardingSlide('assets/onboarding/03.jpg', 'onboarding.s3'),
    OnboardingSlide('assets/onboarding/04.jpg', 'onboarding.s4'),
    OnboardingSlide('assets/onboarding/05.jpg', 'onboarding.s5', soon: true),
    OnboardingSlide('assets/onboarding/06.jpg', 'onboarding.s6'),
  ];

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class OnboardingSlide {
  const OnboardingSlide(this.asset, this.key, {this.soon = false});

  final String asset;

  /// Çelësi bazë: `<key>.title` dhe `<key>.body` në tabelën e teksteve.
  final String key;

  /// Shërbimi nuk është hapur ende; slide-i e thotë hapur (pa premtime të heshtura).
  final bool soon;
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  static const _every = Duration(milliseconds: 4500);

  final _page = PageController();
  Timer? _timer;
  int _index = 0;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _schedule();
  }

  @override
  void dispose() {
    _timer?.cancel();
    _page.dispose();
    super.dispose();
  }

  void _schedule() {
    _timer?.cancel();
    _timer = Timer.periodic(_every, (_) {
      if (!mounted || !_page.hasClients) return;
      final next = (_index + 1) % OnboardingScreen.slides.length;
      _page.animateToPage(
        next,
        duration: const Duration(milliseconds: 520),
        curve: Curves.easeInOutCubic,
      );
    });
  }

  Future<void> _start() async {
    if (_busy) return;
    setState(() => _busy = true);
    final state = context.read<AppState>();
    await state.completeLanguage(state.locale);
  }

  @override
  Widget build(BuildContext context) {
    final state = context.watch<AppState>();
    final slides = OnboardingScreen.slides;
    final slide = slides[_index];
    final bottom = MediaQuery.paddingOf(context).bottom;

    return Scaffold(
      backgroundColor: K.bg,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // Fotot: rrëshqitje me gisht ose vetë; përdoruesi që prek e ndal ritmin për pak.
          NotificationListener<ScrollNotification>(
            onNotification: (n) {
              if (n is ScrollStartNotification && n.dragDetails != null) _timer?.cancel();
              if (n is ScrollEndNotification) _schedule();
              return false;
            },
            child: PageView.builder(
              controller: _page,
              itemCount: slides.length,
              onPageChanged: (i) => setState(() => _index = i),
              itemBuilder: (_, i) => Image.asset(
                slides[i].asset,
                fit: BoxFit.cover,
                alignment: Alignment.center,
                gaplessPlayback: true,
                excludeFromSemantics: true,
              ),
            ),
          ),
          // Errësim lart (për logon) dhe poshtë (për tekstin dhe butonin).
          const IgnorePointer(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  stops: [0, 0.18, 0.5, 0.78, 1],
                  colors: [
                    Color(0xB30D0D0D),
                    Color(0x000D0D0D),
                    Color(0x000D0D0D),
                    Color(0xE60D0D0D),
                    Color(0xFF0D0D0D),
                  ],
                ),
              ),
            ),
          ),
          SafeArea(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, 0),
              child: Row(
                children: [
                  const KWordmark(size: 26, animate: false),
                  const Spacer(),
                  KLangSwitch(value: state.locale, onChanged: (code) => state.previewLocale(code)),
                ],
              ),
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: Padding(
              padding: EdgeInsets.fromLTRB(K.s5, 0, K.s5, bottom + K.s5),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  AnimatedSwitcher(
                    duration: const Duration(milliseconds: 260),
                    switchInCurve: Curves.easeOut,
                    switchOutCurve: Curves.easeIn,
                    layoutBuilder: (current, previous) =>
                        Stack(alignment: Alignment.bottomLeft, children: [...previous, ?current]),
                    child: Column(
                      key: ValueKey(_index),
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (slide.soon) ...[
                          KChip(context.t('common.soon')),
                          const SizedBox(height: K.s3),
                        ],
                        Text(
                          context.t('${slide.key}.title'),
                          style: const TextStyle(
                            fontSize: 30,
                            fontWeight: FontWeight.w800,
                            color: K.text,
                            height: 1.1,
                            letterSpacing: -0.4,
                          ),
                        ),
                        const SizedBox(height: K.s2),
                        Text(
                          context.t('${slide.key}.body'),
                          style: const TextStyle(fontSize: 15, color: K.textDim, height: 1.45),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: K.s5),
                  Row(
                    children: [
                      for (var i = 0; i < slides.length; i++)
                        AnimatedContainer(
                          duration: const Duration(milliseconds: 260),
                          margin: const EdgeInsets.only(right: 6),
                          width: i == _index ? 22 : 6,
                          height: 6,
                          decoration: BoxDecoration(
                            color: i == _index ? K.brand500 : K.line2,
                            borderRadius: BorderRadius.circular(K.rFull),
                            boxShadow: i == _index
                                ? [
                                    BoxShadow(
                                      color: K.brand500.withValues(alpha: 0.5),
                                      blurRadius: 8,
                                    ),
                                  ]
                                : null,
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: K.s5),
                  KButton(
                    label: context.t('onboarding.start'),
                    busy: _busy,
                    onPressed: _busy ? null : _start,
                  ),
                  const SizedBox(height: K.s3),
                  Text(
                    context.t('onboarding.footer'),
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
