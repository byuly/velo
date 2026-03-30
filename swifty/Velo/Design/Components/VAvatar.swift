import SwiftUI

struct VAvatar: View {
    let displayName: String
    var size: CGFloat = 40
    var background: Color = Tokens.Colors.primary

    private var initials: String {
        let parts = displayName.split(separator: " ")
        if parts.count >= 2 {
            return "\(parts[0].prefix(1))\(parts[1].prefix(1))".uppercased()
        }
        return String(displayName.prefix(1)).uppercased()
    }

    private var fontSize: CGFloat { size * 0.36 }

    var body: some View {
        Circle()
            .fill(background)
            .frame(width: size, height: size)
            .overlay(
                Text(initials)
                    .font(.system(size: fontSize, weight: .semibold))
                    .foregroundStyle(.white)
            )
    }
}

struct VInviteCircle: View {
    var size: CGFloat = 40

    var body: some View {
        Circle()
            .strokeBorder(style: StrokeStyle(lineWidth: 2, dash: [4, 3]))
            .foregroundStyle(Tokens.Colors.border)
            .frame(width: size, height: size)
            .overlay(
                Image(systemName: "plus")
                    .font(.system(size: size * 0.3, weight: .medium))
                    .foregroundStyle(Tokens.Colors.mutedForeground)
            )
    }
}

#Preview {
    HStack(spacing: 12) {
        VAvatar(displayName: "Alex")
        VAvatar(displayName: "Sam Jordan", size: 48)
        VInviteCircle(size: 40)
    }
    .padding()
}
