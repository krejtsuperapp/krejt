import 'package:flutter/widgets.dart';

/// Tokenët e markës KREJT (§9, §10) — identiteti zyrtar (04.09.2026): E ZEZË + JESHILE NEON +
/// E BARDHË + GRI TË ERRËTA. Vetëm dark mode: marka jeton mbi të zezë; neoni është theks, jo sfond.
/// Vlerat vijnë nga tabela e markës (Neon Green #39FF14, Jet Black #0D0D0D, Dark Gray #1A1A1A,
/// Medium Gray #2E2E2E, tekst dytësor #AFAFAF) dhe ndahen me panelet (globals.css).
class K {
  const K._();

  // --- marka: jeshile neon ---------------------------------------------------------
  // Shkalla e errët (50–300) shërben për sfonde të theksuara; 400 për tekst/ikona mbi të zezë
  // (më e lehtë, lexohet); 500 është neoni zyrtar për butona/tregues; 600–700 për gjendje shtypjeje.
  static const brand50 = Color(0xFF0C2607);
  static const brand100 = Color(0xFF123A0A);
  static const brand200 = Color(0xFF1B5A0F);
  static const brand300 = Color(0xFF268A14);
  static const brand400 = Color(0xFF5CFF3D);
  static const brand500 = Color(0xFF39FF14); // primar — Neon Green
  static const brand600 = Color(0xFF2ED60F);
  static const brand700 = Color(0xFF23A80B);

  /// Theksi i dytë: neon i zbutur për fundin e gradientit dhe për tekst mbi sfonde jeshile të errëta.
  static const accent2 = Color(0xFFB8FF9E);

  /// Mbi neon shkruhet me të zezë: e bardha nuk lexohet mbi #39FF14 (kontrast ~1.2:1).
  static const onBrand = Color(0xFF0D0D0D);

  /// Gradient i përmbajtur (neon → neon pak më i errët): butoni duket i ngopur, jo "i ndezur".
  static const gradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [brand500, brand600],
  );

  // --- sipërfaqet dhe teksti ---------------------------------------------------------
  static const bg = Color(0xFF0D0D0D); // Jet Black
  static const surface = Color(0xFF1A1A1A); // Dark Gray — kartat
  static const surface2 = Color(0xFF222222); // fushat, chip-at
  static const surface3 = Color(0xFF2E2E2E); // Medium Gray — snackbar, ngritje
  static const text = Color(0xFFFFFFFF);
  static const textDim = Color(0xFFD9D9D9);
  static const muted = Color(0xFFAFAFAF); // tekst dytësor
  static const line = Color(0xFF2E2E2E);
  static const line2 = Color(0xFF3D3D3D);

  // --- semantika (e ndarë nga theksi i markës) ------------------------------------------
  static const ok = Color(0xFF22C55E);
  static const okBg = Color(0xFF0F2A1A);
  static const warn = Color(0xFFF79009);
  static const warnBg = Color(0xFF33240C);
  static const danger = Color(0xFFF04438);
  static const dangerBg = Color(0xFF331615);
  static const info = Color(0xFF2E90FA);
  static const infoBg = Color(0xFF0C2033);

  // --- tipografia: Inter (e paketuar në krejt_design) ------------------------------------
  static const fontFamily = 'Inter';
  static const fontPackage = 'krejt_design';

  /// Emri i familjes siç e kërkon ThemeData jashtë paketës.
  static const themeFontFamily = 'packages/krejt_design/Inter';

  // --- rrezet ------------------------------------------------------------------------
  static const rXs = 6.0;
  static const rSm = 10.0;
  static const rMd = 14.0;
  static const rLg = 20.0;
  static const rXl = 28.0;
  static const rFull = 999.0;

  // --- hapësira (rrjeti 4 pt) ----------------------------------------------------------
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
