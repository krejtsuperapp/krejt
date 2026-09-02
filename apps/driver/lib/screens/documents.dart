import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Llojet që kërkon shqyrtimi, sipas radhës në të cilën i mbledh Operacionet (§31).
const documentTypes = [
  'driving_license',
  'id_card',
  'vehicle_registration',
  'insurance',
  'criminal_record',
  'profile_photo',
];

String documentTypeKey(String type) => 'driver.docs.type.$type';

String documentStatusKey(DocumentStatus s) => 'driver.docs.status.${s.name}';

KTone documentTone(DocumentStatus s) {
  switch (s) {
    case DocumentStatus.approved:
      return KTone.ok;
    case DocumentStatus.rejected:
    case DocumentStatus.expired:
      return KTone.danger;
    case DocumentStatus.pending:
      return KTone.info;
    case DocumentStatus.replaced:
      return KTone.neutral;
  }
}

/// Dokumentet e shoferit. Fotoja shkon drejt në magazinë me një URL të nënshkruar që skadon;
/// serveri e konfirmon dhe e vë në radhë për shqyrtim (§31).
class DocumentsScreen extends StatefulWidget {
  const DocumentsScreen({super.key});

  @override
  State<DocumentsScreen> createState() => _DocumentsScreenState();
}

class _DocumentsScreenState extends State<DocumentsScreen> {
  DocumentsOverview? _overview;
  bool _loading = true;
  String? _uploading;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final overview = await context.read<AppState>().api.driverDocuments();
      if (!mounted) return;
      setState(() {
        _overview = overview;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  Future<void> _upload(String type) async {
    final source = await showKSheet<ImageSource>(
      context: context,
      title: context.t(documentTypeKey(type)),
      child: Column(
        children: [
          KButton(
            label: context.t('driver.docs.camera'),
            icon: Icons.photo_camera_outlined,
            onPressed: () => Navigator.of(context).pop(ImageSource.camera),
          ),
          const SizedBox(height: K.s2),
          KOutlineButton(
            label: context.t('driver.docs.gallery'),
            icon: Icons.photo_library_outlined,
            onPressed: () => Navigator.of(context).pop(ImageSource.gallery),
          ),
        ],
      ),
    );
    if (source == null || !mounted) return;

    final picked = await ImagePicker().pickImage(source: source, maxWidth: 2000, imageQuality: 85);
    if (picked == null || !mounted) return;

    setState(() => _uploading = type);
    final api = context.read<AppState>().api;
    final messenger = ScaffoldMessenger.of(context);
    final done = context.t('driver.docs.uploaded');
    try {
      final bytes = await picked.readAsBytes();
      await api.uploadDriverDocument(type: type, bytes: bytes, contentType: 'image/jpeg');
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(done)));
      await _load();
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _uploading = null);
    }
  }

  DriverDocument? _current(String type) {
    final docs = _overview?.documents ?? const <DriverDocument>[];
    for (final d in docs) {
      if (d.type == type && d.status != DocumentStatus.replaced) return d;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('driver.docs.title'))),
      body: SafeArea(child: _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 80));
    }
    final overview = _overview;
    if (overview == null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error?.messageKey ?? 'errors.internal'),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _load,
      color: K.brand400,
      backgroundColor: K.surface2,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
        children: [
          if (overview.eligible)
            KCard(
              child: Row(
                children: [
                  const Icon(Icons.verified_outlined, size: 20, color: K.ok),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      context.t('driver.docs.eligible'),
                      style: const TextStyle(fontSize: 14, color: K.textDim),
                    ),
                  ),
                ],
              ),
            )
          else
            KCard(
              child: Text(
                context.t('driver.documents.missing', {'n': '${overview.missing.length}'}),
                style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.4),
              ),
            ),
          const SizedBox(height: K.s5),
          for (final type in documentTypes)
            Padding(
              padding: const EdgeInsets.only(bottom: K.s2),
              child: _DocumentRow(
                type: type,
                document: _current(type),
                expiring: overview.expiring.contains(type),
                busy: _uploading == type,
                onUpload: () => _upload(type),
              ),
            ),
        ],
      ),
    );
  }
}

class _DocumentRow extends StatelessWidget {
  const _DocumentRow({
    required this.type,
    required this.document,
    required this.expiring,
    required this.busy,
    required this.onUpload,
  });

  final String type;
  final DriverDocument? document;
  final bool expiring;
  final bool busy;
  final VoidCallback onUpload;

  @override
  Widget build(BuildContext context) {
    final doc = document;
    return KCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  context.t(documentTypeKey(type)),
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              if (doc != null)
                KBadge(context.t(documentStatusKey(doc.status)), tone: documentTone(doc.status)),
            ],
          ),
          if (expiring) ...[
            const SizedBox(height: K.s1),
            KBadge(context.t('driver.docs.expiring'), tone: KTone.warn),
          ],
          if (doc?.rejectionReason != null) ...[
            const SizedBox(height: K.s2),
            Text(
              doc!.rejectionReason!,
              style: const TextStyle(fontSize: 13, color: K.danger, height: 1.4),
            ),
          ],
          const SizedBox(height: K.s3),
          KOutlineButton(
            label: context.t(doc == null ? 'driver.docs.upload' : 'driver.docs.replace'),
            icon: Icons.upload_outlined,
            onPressed: busy ? null : onUpload,
          ),
        ],
      ),
    );
  }
}
