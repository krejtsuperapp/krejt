import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';

/// Llojet e raportimit, në të njëjtën radhë si te serveri. `sos` rri i pari sepse është ai që
/// kërkohet me nxitim; të tjerat vijnë sipas sa shpesh ndodhin.
const _kinds = <String>[
  'sos',
  'unsafe_driving',
  'harassment',
  'accident',
  'vehicle_issue',
  'other',
];

/// Ndihma dhe siguria gjatë një udhëtimi. Serveri e mbante `/api/v1/safety/reports` prej fillimi
/// dhe hap një tiketë urgjente te operacionet, por asnjë ekran nuk e thërriste: gjatë udhëtimit
/// përdoruesi nuk kishte asnjë buton përveç anulimit, i cili as nuk shfaqet pasi udhëtimi nis.
///
/// Me qëllim nuk premtohet ndihmë emergjente: raporti shkon te operacionet e KREJT-it, jo te
/// policia. Teksti e thotë hapur, që askush të mos presë ambulancën nga një buton në telefon.
/// [at] jepet nga aplikacioni thirrës, sepse vendndodhja merret ndryshe te klienti dhe te shoferi.
/// Null nuk e ndal raportin: askush nuk duhet të mbetet pa mundësi raportimi për shkak të një lejeje.
Future<void> showSafetySheet(
  BuildContext context, {
  required KrejtApi api,
  required String rideId,
  LatLng? at,
}) async {
  final kind = await showKSheet<String>(
    context: context,
    title: context.t('safety.title'),
    subtitle: context.t('safety.subtitle'),
    scrollable: true,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final k in _kinds)
          Padding(
            padding: const EdgeInsets.only(bottom: K.s2),
            child: KCard(
              onTap: () => Navigator.of(context).pop(k),
              padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s4),
              child: Row(
                children: [
                  Icon(
                    k == 'sos' ? Icons.emergency_outlined : Icons.report_outlined,
                    size: 20,
                    color: k == 'sos' ? K.danger : K.textDim,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      context.t('safety.kind.$k'),
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                        color: k == 'sos' ? K.danger : K.text,
                      ),
                    ),
                  ),
                  const Icon(Icons.chevron_right, size: 20, color: K.line2),
                ],
              ),
            ),
          ),
      ],
    ),
  );
  if (kind == null || !context.mounted) return;
  await _confirmAndSend(context, api: api, rideId: rideId, kind: kind, at: at);
}

Future<void> _confirmAndSend(
  BuildContext context, {
  required KrejtApi api,
  required String rideId,
  required String kind,
  LatLng? at,
}) async {
  final note = TextEditingController();
  final send = await showKSheet<bool>(
    context: context,
    title: context.t('safety.kind.$kind'),
    subtitle: context.t('safety.confirm'),
    scrollable: true,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        KField(controller: note, label: context.t('safety.note'), maxLines: 3, maxLength: 500),
        const SizedBox(height: K.s4),
        KButton(label: context.t('safety.send'), onPressed: () => Navigator.of(context).pop(true)),
      ],
    ),
  );
  if (send != true || !context.mounted) {
    note.dispose();
    return;
  }
  final text = note.text.trim();
  note.dispose();

  final messenger = ScaffoldMessenger.of(context);
  final l10n = KL10n.of(context);
  final sent = l10n.t('safety.sent');
  try {
    await api.reportSafety(
      kind: kind,
      rideId: rideId,
      description: text.isEmpty ? null : text,
      at: at,
    );
    messenger.showSnackBar(SnackBar(content: Text(sent)));
  } on ApiError catch (e) {
    messenger.showSnackBar(SnackBar(content: Text(l10n.error(e.messageKey))));
  }
}
