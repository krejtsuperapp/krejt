import 'package:flutter/material.dart';

import '../tokens.dart';

/// Fleta e poshtme është kontejneri kryesor i vendimeve në KREJT: zgjedhja e kategorisë,
/// konfirmimi i pagesës, anulimi, vlerësimi. Titulli dhe doreza janë gjithmonë të pranishme
/// që përdoruesi ta dijë ku ndodhet dhe si të kthehet (§55).
class KSheet extends StatelessWidget {
  const KSheet({
    super.key,
    required this.child,
    this.title,
    this.subtitle,
    this.actions,
    this.padding = const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s5),
    this.scrollable = false,
  });

  final Widget child;
  final String? title;
  final String? subtitle;

  /// Butonat e fundit, të ngjitur poshtë përmbajtjes.
  final Widget? actions;
  final EdgeInsets padding;
  final bool scrollable;

  @override
  Widget build(BuildContext context) {
    final media = MediaQuery.of(context);
    final content = Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (title != null) ...[
          Text(
            title!,
            style: const TextStyle(fontSize: 19, fontWeight: FontWeight.w700, color: K.text),
          ),
          if (subtitle != null)
            Padding(
              padding: const EdgeInsets.only(top: K.s1),
              child: Text(subtitle!, style: const TextStyle(fontSize: 14, color: K.muted)),
            ),
          const SizedBox(height: K.s4),
        ],
        child,
        if (actions != null) ...[const SizedBox(height: K.s5), actions!],
      ],
    );

    return SafeArea(
      top: false,
      child: Container(
        decoration: const BoxDecoration(
          color: K.surface,
          border: Border(top: BorderSide(color: K.line)),
          borderRadius: BorderRadius.vertical(top: Radius.circular(K.rXl)),
        ),
        padding: EdgeInsets.only(bottom: media.viewInsets.bottom),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const KSheetHandle(),
            Flexible(
              child: scrollable
                  ? SingleChildScrollView(padding: padding, child: content)
                  : Padding(padding: padding, child: content),
            ),
          ],
        ),
      ),
    );
  }
}

class KSheetHandle extends StatelessWidget {
  const KSheetHandle({super.key});

  @override
  Widget build(BuildContext context) => Container(
    width: 40,
    height: 4,
    margin: const EdgeInsets.only(top: K.s3, bottom: K.s2),
    decoration: BoxDecoration(color: K.line2, borderRadius: BorderRadius.circular(K.rFull)),
  );
}

/// Hap një [KSheet] me sjelljen e njëjtë kudo: mbyllet me tërheqje, respekton tastierën,
/// dhe kthen vlerën e zgjedhur.
Future<T?> showKSheet<T>({
  required BuildContext context,
  required Widget child,
  String? title,
  String? subtitle,
  Widget? actions,
  bool scrollable = false,
  bool dismissible = true,
}) {
  return showModalBottomSheet<T>(
    context: context,
    isScrollControlled: true,
    isDismissible: dismissible,
    enableDrag: dismissible,
    backgroundColor: Colors.transparent,
    barrierColor: Colors.black.withValues(alpha: 0.62),
    constraints: BoxConstraints(maxHeight: MediaQuery.of(context).size.height * 0.92),
    builder: (_) => KSheet(
      title: title,
      subtitle: subtitle,
      actions: actions,
      scrollable: scrollable,
      child: child,
    ),
  );
}

/// Dialog konfirmimi për veprime që kushtojnë para ose nuk kthehen (anulim me tarifë, dalje nga llogaria).
Future<bool> confirmKSheet({
  required BuildContext context,
  required String title,
  required String message,
  required String confirmLabel,
  String? cancelLabel,
  bool destructive = false,
}) async {
  final res = await showKSheet<bool>(
    context: context,
    title: title,
    child: Text(message, style: const TextStyle(fontSize: 15, color: K.textDim, height: 1.45)),
    actions: Column(
      children: [
        SizedBox(
          width: double.infinity,
          height: K.minTap,
          child: FilledButton(
            style: destructive
                ? FilledButton.styleFrom(backgroundColor: K.danger, foregroundColor: Colors.white)
                : null,
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(confirmLabel),
          ),
        ),
        const SizedBox(height: K.s2),
        SizedBox(
          width: double.infinity,
          height: K.minTap,
          child: TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(cancelLabel ?? 'Jo'),
          ),
        ),
      ],
    ),
  );
  return res == true;
}
