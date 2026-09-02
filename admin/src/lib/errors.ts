import { ApiError } from './api';

/// Tekstet e gabimeve, të lidhura me `message_key` që dërgon serveri. Përdoruesi nuk sheh kurrë
/// tekstin e papërpunuar të serverit, dhe një çelës i panjohur bie te mesazhi i përgjithshëm (§55).
const messages: Record<string, string> = {
  'errors.internal': 'Diçka shkoi keq. Provo përsëri.',
  'errors.offline': 'Nuk ka lidhje me serverin.',
  'errors.unauthorized': 'Sesioni skadoi. Kyçu përsëri.',
  'errors.forbidden': 'Nuk ke të drejta për këtë veprim.',
  'errors.not_found': 'Nuk u gjet.',
  'errors.validation': 'Kontrollo të dhënat e shkruara.',
  'errors.rate_limited': 'Shumë përpjekje. Prit pak.',
  'errors.otp_invalid': 'Kodi nuk përputhet ose ka skaduar.',
  'errors.documents_missing': 'Shoferi ende nuk i ka të gjitha dokumentet e aprovuara.',
  'errors.insufficient_funds': 'Wallet-i nuk ka mjaftueshëm për këtë rimbursim.',
};

export function errorText(e: unknown): string {
  if (e instanceof ApiError) {
    return messages[e.messageKey] ?? messages['errors.internal'];
  }
  // Dështimi i rrjetit nuk vjen me zarf gabimi.
  return messages['errors.offline'];
}
