import SwiftUI

enum VBadgeVariant {
    case active, complete, failed, generating, cancelled, neutral

    var background: Color {
        switch self {
        case .active: return Tokens.Colors.primaryLight
        case .complete: return Tokens.Colors.successLight
        case .failed: return Tokens.Colors.destructiveLight
        case .generating: return Tokens.Colors.warningLight
        case .cancelled: return Tokens.Colors.secondary
        case .neutral: return Tokens.Colors.secondary
        }
    }

    var foreground: Color {
        switch self {
        case .active: return Tokens.Colors.primary
        case .complete: return Tokens.Colors.success
        case .failed: return Tokens.Colors.destructive
        case .generating: return Tokens.Colors.warning
        case .cancelled: return Tokens.Colors.mutedForeground
        case .neutral: return Tokens.Colors.secondaryForeground
        }
    }

    var dot: Bool {
        switch self {
        case .active: return true
        default: return false
        }
    }
}

extension SessionStatus {
    var badgeVariant: VBadgeVariant {
        switch self {
        case .active: return .active
        case .complete: return .complete
        case .failed: return .failed
        case .generating: return .generating
        case .cancelled: return .cancelled
        }
    }

    var label: String {
        switch self {
        case .active: return "Active"
        case .complete: return "Complete"
        case .failed: return "Failed"
        case .generating: return "Generating…"
        case .cancelled: return "Cancelled"
        }
    }
}

struct VBadge: View {
    let text: String
    var variant: VBadgeVariant = .neutral

    var body: some View {
        HStack(spacing: 5) {
            if variant.dot {
                Circle()
                    .fill(variant.foreground)
                    .frame(width: 6, height: 6)
            }
            Text(text)
                .font(Tokens.Typography.caption)
                .fontWeight(.semibold)
                .foregroundStyle(variant.foreground)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 4)
        .background(variant.background)
        .clipShape(Capsule())
    }
}

#Preview {
    HStack(spacing: 8) {
        VBadge(text: "Active", variant: .active)
        VBadge(text: "Complete", variant: .complete)
        VBadge(text: "Failed", variant: .failed)
        VBadge(text: "Generating…", variant: .generating)
    }
    .padding()
}
