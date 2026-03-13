import SwiftUI

struct SessionCard: View {
    let session: Session

    var body: some View {
        VCard {
                VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                    HStack {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(session.name)
                                .font(Tokens.Typography.h3)
                                .foregroundStyle(Tokens.Colors.foreground)
                                .lineLimit(1)

                            Text(formattedDate)
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                        }

                        Spacer()

                        VBadge(
                            text: session.status.label,
                            variant: session.status.badgeVariant
                        )
                    }

                    // Participants row
                    HStack(spacing: -8) {
                        ForEach(session.participants) { participant in
                            VAvatar(displayName: participant.displayName, size: 32)
                                .overlay(
                                    Circle()
                                        .stroke(Tokens.Colors.background, lineWidth: 2)
                                )
                        }
                    }

                    // Footer
                    if session.status == .active {
                        HStack {
                            Image(systemName: "clock")
                                .font(.system(size: 12))
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                            Text("Ends in \(session.timeUntilDeadline)")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                            Spacer()
                            Text("\(session.slots.count) slots")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.mutedForeground)
                        }
                    } else if session.status == .complete {
                        HStack {
                            Image(systemName: "play.circle.fill")
                                .font(.system(size: 12))
                                .foregroundStyle(Tokens.Colors.primary)
                            Text("Tap to watch reel")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Tokens.Colors.primary)
                                .fontWeight(.medium)
                        }
                    }

                    // Expiry warning
                    if session.isExpiringSoon {
                        HStack(spacing: Tokens.Spacing.xs) {
                            Image(systemName: "exclamationmark.triangle.fill")
                                .font(.system(size: 11))
                                .foregroundStyle(Tokens.Colors.warning)
                            Text("This reel expires in 15 days. Save it to keep it forever.")
                                .font(Tokens.Typography.caption)
                                .foregroundStyle(Tokens.Colors.warning)
                        }
                        .padding(Tokens.Spacing.sm)
                        .background(Tokens.Colors.warningLight)
                        .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.sm))
                    }
                }
        }
    }

    private var formattedDate: String {
        let formatter = DateFormatter()
        formatter.dateStyle = .medium
        formatter.timeStyle = .none
        return formatter.string(from: session.createdAt)
    }
}

#Preview {
    VStack(spacing: 12) {
        SessionCard(session: MockData.sessions[0])
        SessionCard(session: MockData.sessions[1])
    }
    .padding()
}
