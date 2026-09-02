import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import 'tokens.dart';

/// Tema e vetme e KREJT (dark). Të gjitha aplikacionet e përdorin këtë — asnjë ngjyrë e ngurtë në ekrane.
ThemeData krejtTheme() {
  const scheme = ColorScheme.dark(
    primary: K.brand500,
    onPrimary: K.onBrand,
    secondary: K.violet,
    onSecondary: K.onBrand,
    surface: K.surface,
    onSurface: K.text,
    error: K.danger,
    onError: K.onBrand,
    outline: K.line,
  );

  final base = ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: scheme,
    scaffoldBackgroundColor: K.bg,
    canvasColor: K.bg,
    splashFactory: InkSparkle.splashFactory,
    visualDensity: VisualDensity.standard,
  );

  return base.copyWith(
    textTheme: _text(base.textTheme),
    appBarTheme: const AppBarTheme(
      backgroundColor: K.bg,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      centerTitle: false,
      titleTextStyle: TextStyle(
        color: K.text,
        fontSize: 20,
        fontWeight: FontWeight.w700,
        letterSpacing: -0.2,
      ),
      iconTheme: IconThemeData(color: K.text),
      systemOverlayStyle: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.light,
        statusBarBrightness: Brightness.dark,
      ),
    ),
    cardTheme: CardThemeData(
      color: K.surface,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(K.rMd),
        side: const BorderSide(color: K.line),
      ),
    ),
    dividerTheme: const DividerThemeData(color: K.line, thickness: 1, space: 1),
    bottomNavigationBarTheme: const BottomNavigationBarThemeData(
      backgroundColor: K.surface,
      selectedItemColor: K.brand400,
      unselectedItemColor: K.muted,
      type: BottomNavigationBarType.fixed,
      showUnselectedLabels: true,
    ),
    navigationBarTheme: NavigationBarThemeData(
      backgroundColor: K.surface,
      surfaceTintColor: Colors.transparent,
      indicatorColor: K.brand500.withValues(alpha: 0.18),
      height: 68,
      labelTextStyle: WidgetStateProperty.resolveWith(
        (s) => TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: s.contains(WidgetState.selected) ? K.brand400 : K.muted,
        ),
      ),
      iconTheme: WidgetStateProperty.resolveWith(
        (s) =>
            IconThemeData(color: s.contains(WidgetState.selected) ? K.brand400 : K.muted, size: 24),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: K.surface2,
      contentPadding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s4),
      hintStyle: const TextStyle(color: K.muted),
      labelStyle: const TextStyle(color: K.textDim),
      border: _border(K.line),
      enabledBorder: _border(K.line),
      focusedBorder: _border(K.brand500, width: 1.6),
      errorBorder: _border(K.danger),
      focusedErrorBorder: _border(K.danger, width: 1.6),
      errorStyle: const TextStyle(color: K.danger, fontSize: 12.5, height: 1.3),
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: K.surface3,
      contentTextStyle: const TextStyle(color: K.text),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(K.rSm)),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: K.surface,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(K.rLg)),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: K.surface,
      surfaceTintColor: Colors.transparent,
      showDragHandle: true,
      dragHandleColor: K.line2,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(K.rXl)),
      ),
    ),
    chipTheme: ChipThemeData(
      backgroundColor: K.surface2,
      side: const BorderSide(color: K.line),
      labelStyle: const TextStyle(color: K.textDim, fontWeight: FontWeight.w600, fontSize: 13),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(K.rFull)),
    ),
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: K.brand400,
      linearTrackColor: K.surface3,
    ),
    listTileTheme: const ListTileThemeData(iconColor: K.muted, textColor: K.text),
  );
}

OutlineInputBorder _border(Color c, {double width = 1}) => OutlineInputBorder(
  borderRadius: BorderRadius.circular(K.rMd),
  borderSide: BorderSide(color: c, width: width),
);

TextTheme _text(TextTheme t) => t.copyWith(
  displaySmall: t.displaySmall?.copyWith(
    color: K.text,
    fontWeight: FontWeight.w800,
    letterSpacing: -0.6,
  ),
  headlineMedium: t.headlineMedium?.copyWith(
    color: K.text,
    fontWeight: FontWeight.w800,
    letterSpacing: -0.4,
  ),
  headlineSmall: t.headlineSmall?.copyWith(
    color: K.text,
    fontWeight: FontWeight.w700,
    letterSpacing: -0.3,
  ),
  titleLarge: t.titleLarge?.copyWith(
    color: K.text,
    fontWeight: FontWeight.w700,
    letterSpacing: -0.2,
  ),
  titleMedium: t.titleMedium?.copyWith(color: K.text, fontWeight: FontWeight.w600),
  titleSmall: t.titleSmall?.copyWith(color: K.textDim, fontWeight: FontWeight.w600),
  bodyLarge: t.bodyLarge?.copyWith(color: K.text, height: 1.45),
  bodyMedium: t.bodyMedium?.copyWith(color: K.textDim, height: 1.45),
  bodySmall: t.bodySmall?.copyWith(color: K.muted, height: 1.4),
  labelLarge: t.labelLarge?.copyWith(fontWeight: FontWeight.w700, letterSpacing: 0.1),
  labelSmall: t.labelSmall?.copyWith(color: K.muted, letterSpacing: 0.4),
);
