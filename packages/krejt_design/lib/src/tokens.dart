import 'package:flutter/widgets.dart';

/// Tokenët e markës KREJT (§9, §10). Vetëm dark mode — vendim i marrë më 02.09.2026.
/// Vlerat janë identike me `krejt/css/tokens.css`, që mockup-i dhe aplikacioni të mos ndahen kurrë.
class K {
  const K._();

  // --- marka: vjollcë → magenta ------------------------------------------------
  static const brand50 = Color(0xFF1B1538);
  static const brand100 = Color(0xFF241C4C);
  static const brand200 = Color(0xFF33266E);
  static const brand300 = Color(0xFF4A34A0);
  static const brand400 = Color(0xFF855EFF);
  static const brand500 = Color(0xFF6A3DFF); // primar
  static const brand600 = Color(0xFF562FD6);
  static const brand700 = Color(0xFF4324AB);
  static const violet = Color(0xFFC23DFF); // fundi i gradientit
  static const onBrand = Color(0xFFFFFFFF);

  static const gradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [brand500, violet],
  );

  // --- sipërfaqet dhe teksti ---------------------------------------------------
  static const bg = Color(0xFF070B18);
  static const surface = Color(0xFF0F1526);
  static const surface2 = Color(0xFF141B30);
  static const surface3 = Color(0xFF1B2440);
  static const text = Color(0xFFF2F5FF);
  static const textDim = Color(0xFFC3CBE4);
  static const muted = Color(0xFF8E9AB9);
  static const line = Color(0xFF212B47);
  static const line2 = Color(0xFF2C385A);

  // --- semantika ---------------------------------------------------------------
  static const ok = Color(0xFF12B76A);
  static const okBg = Color(0xFF0C2B1F);
  static const warn = Color(0xFFF79009);
  static const warnBg = Color(0xFF33240C);
  static const danger = Color(0xFFF04438);
  static const dangerBg = Color(0xFF331615);
  static const info = Color(0xFF2E90FA);
  static const infoBg = Color(0xFF0C2033);

  // --- rrezet ------------------------------------------------------------------
  static const rXs = 6.0;
  static const rSm = 10.0;
  static const rMd = 14.0;
  static const rLg = 20.0;
  static const rXl = 28.0;
  static const rFull = 999.0;

  // --- hapësira (rrjeti 4 pt) --------------------------------------------------
  static const s1 = 4.0;
  static const s2 = 8.0;
  static const s3 = 12.0;
  static const s4 = 16.0;
  static const s5 = 20.0;
  static const s6 = 24.0;
  static const s8 = 32.0;
  static const s10 = 40.0;

  /// Lartësia minimale e prekjes (§56 aksesueshmëria).
  static const minTap = 48.0;
}
